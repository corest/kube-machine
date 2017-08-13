package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/kube-node/kube-machine/pkg/controller"
	"github.com/kube-node/kube-machine/pkg/libmachine"
	"github.com/kube-node/kube-machine/pkg/metrics"
	"github.com/kube-node/kube-machine/pkg/nodeclass"
	"github.com/kube-node/nodeset/pkg/nodeset/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	nodeInformer      cache.Controller
	nodeIndexer       cache.Indexer
	nodeQueue         workqueue.RateLimitingInterface
	nodeClassStore    cache.Store
	nodeClassInformer cache.Controller
	mapi              *libmachine.Client
	client            *kubernetes.Clientset
}

const (
	classAnnotationKey      = "node.k8s.io/node-class"
	phaseAnnotationKey      = "node.k8s.io/state"
	driverDataAnnotationKey = "node.k8s.io/driver-data"
	deleteFinalizerName     = "node.k8s.io/delete"
	publicIPAnnotationKey   = "node.k8s.io/public-ip"
	hostnameAnnotationKey   = "node.k8s.io/hostname"

	phasePending      = "pending"
	phaseProvisioning = "provisioning"
	phaseLaunching    = "launching"
	phaseRunning      = "running"
	phaseDeleting     = "deleting"
)

var NodeClassNotFoundErr = errors.New("node class not found")

func New(
	client *kubernetes.Clientset,
	queue workqueue.RateLimitingInterface,
	nodeIndexer cache.Indexer,
	nodeInformer cache.Controller,
	nodeClassStore cache.Store,
	nodeClassController cache.Controller,
	mapi *libmachine.Client,
) controller.Interface {
	return &Controller{
		nodeInformer:      nodeInformer,
		nodeIndexer:       nodeIndexer,
		nodeQueue:         queue,
		nodeClassInformer: nodeClassController,
		nodeClassStore:    nodeClassStore,
		mapi:              mapi,
		client:            client,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working nodeQueue
	key, quit := c.nodeQueue.Get()
	if quit {
		return false
	}

	defer c.nodeQueue.Done(key)

	// Invoke the method containing the business logic
	err := c.syncNode(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

func (c *Controller) syncNode(key string) (err error) {
	defer func() {
		if err != nil {
			metrics.IncErrors(metrics.Error)
		}
	}()

	nobj, exists, err := c.nodeIndexer.GetByKey(key)
	if err != nil {
		metrics.IncErrors(metrics.Error)
		glog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	if !exists {
		metrics.IncErrors(metrics.Info)
		glog.Infof("Node %s does not exist anymore\n", key)
		return nil
	}

	node := nobj.(*v1.Node)
	originalData, err := json.Marshal(node)

	glog.V(4).Infof("Processing Node %s\n", node.GetName())

	// Get phase of node. In case we have not touched it set phase to `pending`
	phase := node.Annotations[phaseAnnotationKey]
	if phase == "" {
		phase = phasePending
	}

	if node.DeletionTimestamp != nil {
		phase = phaseDeleting
	}
	node.Annotations[phaseAnnotationKey] = phase

	switch phase {
	case phasePending:
		node, err = c.syncPendingNode(node)
	case phaseProvisioning:
		node, err = c.syncProvisioningNode(node)
	case phaseLaunching:
		node, err = c.syncLaunchingNode(node)
	case phaseDeleting:
		node, err = c.syncDeletingNode(node)
	}

	if err != nil {
		return err
	}

	if node != nil {
		modifiedData, err := json.Marshal(node)
		if err != nil {
			return err
		}
		b, err := strategicpatch.CreateTwoWayMergePatch(originalData, modifiedData, v1.Node{})
		if err != nil {
			return err
		}
		//Avoid empty patch calls
		if string(b) == "{}" {
			return nil
		}

		node, err = c.client.Nodes().Patch(node.Name, types.StrategicMergePatchType, b)
		if err != nil {
			return err
		}
		return c.nodeIndexer.Update(node)
	}

	c.nodeQueue.AddAfter(key, 30*time.Second)
	return nil
}

func (c *Controller) getNodeClass(name string) (nc *v1alpha1.NodeClass, ncc *nodeclass.NodeClassConfig, err error) {
	defer func() {
		if err != nil {
			metrics.IncErrors(metrics.Error)
		}
	}()

	ncobj, exists, err := c.nodeClassStore.GetByKey("default/" + name)
	if err != nil {
		return nil, nil, fmt.Errorf("could not fetch nodeclass from store: %v", err)
	}
	if !exists {
		return nil, nil, NodeClassNotFoundErr
	}

	class := ncobj.(*v1alpha1.NodeClass)
	var config nodeclass.NodeClassConfig
	err = json.Unmarshal(class.Config.Raw, &config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not unmarshal config from nodeclass: %v", err)
	}

	return class, &config, nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.nodeQueue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.nodeQueue.NumRequeues(key) < 5 {
		glog.Infof("Error syncing node %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// nodeQueue and the re-enqueue history, the key will be processed later again.
		c.nodeQueue.AddRateLimited(key)
		return
	}

	c.nodeQueue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	glog.Infof("Dropping node %q out of the queue: %v", key, err)
}

func (c *Controller) Run(workerCount int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.nodeQueue.ShutDown()
	glog.Info("Starting Node controller")

	go c.nodeInformer.Run(stopCh)
	go c.nodeClassInformer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the nodeQueue is started
	if !cache.WaitForCacheSync(stopCh, c.nodeInformer.HasSynced, c.nodeClassInformer.HasSynced) {
		runtime.HandleError(errors.New("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workerCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.Info("Stopping Node controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
