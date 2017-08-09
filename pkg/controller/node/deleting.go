package node

import (
	"time"

	"github.com/kube-node/kube-machine/pkg/metrics"
	"k8s.io/client-go/pkg/api/v1"
)

func (c *Controller) syncDeletingNode(node *v1.Node) (changedN *v1.Node, err error) {
	start := time.Now()
	defer func(s time.Time) {
		metrics.SyncOperationTime(time.Now().Sub(s), phaseDeleting)
		if err != nil {
			metrics.IncErrors(metrics.Error)
		}
	}(start)

	changedN, err = c.deleteInstance(node)
	if err != nil || changedN != nil {
		return changedN, err
	}

	metrics.DecNodes()
	return nil, nil
}

func (c *Controller) deleteInstance(node *v1.Node) (n *v1.Node, err error) {
	defer func() {
		if err != nil {
			metrics.IncErrors(metrics.Error)
		}
	}()

	for i, f := range node.Finalizers {
		if f == deleteFinalizerName {
			node.Finalizers = append(node.Finalizers[:i], node.Finalizers[i+1:]...)
			break
		}
	}

	if node.Annotations[driverDataAnnotationKey] == "" {
		return node, nil
	}

	h, err := c.mapi.Load(node)
	if err != nil {
		return nil, err
	}

	err = h.Driver.Remove()
	if err != nil {
		return nil, err
	}

	return node, nil
}
