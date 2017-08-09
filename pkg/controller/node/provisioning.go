package node

import (
	"fmt"
	"time"

	"github.com/kube-node/kube-machine/pkg/metrics"

	"encoding/json"

	"k8s.io/client-go/pkg/api/v1"
)

func (c *Controller) syncProvisioningNode(node *v1.Node) (changedN *v1.Node, err error) {
	start := time.Now()
	defer func(s time.Time) {
		metrics.SyncOperationTime(time.Now().Sub(s), phaseProvisioning)
		if err != nil {
			metrics.IncErrors(metrics.Error)
		}
	}(start)

	changedN, err = c.provisionInstance(node)
	if err != nil || changedN != nil {
		return changedN, err
	}

	return nil, nil
}

func (c *Controller) provisionInstance(node *v1.Node) (n *v1.Node, err error) {
	defer func() {
		if err != nil {
			metrics.IncErrors(metrics.Error)
		}
	}()

	h, err := c.mapi.Load(node)
	if err != nil {
		return nil, err
	}

	_, config, err := c.getNodeClass(node.Annotations[classAnnotationKey])
	if err != nil {
		return nil, fmt.Errorf("could not get nodeclass %q for node %s: %v", node.Annotations[classAnnotationKey], node.Name, err)
	}

	err = c.mapi.Provision(h, config)
	if err != nil {
		return nil, fmt.Errorf("could not provision: %v", err)
	}

	data, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	node.Annotations[driverDataAnnotationKey] = string(data)
	node.Annotations[phaseAnnotationKey] = phaseLaunching

	return node, nil
}
