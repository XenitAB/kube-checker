package check

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/xenitab/kube-checker/pkg/graph"
)

const (
	instanceTypeKey = "node.kubernetes.io/instance-type"
	agentPoolKey    = "kubernetes.azure.com/agentpool"
	classKey        = "xkf.xenit.io/node-class"
	purposeKey      = "xkf.xenit.io/purpose"
	storageTierKey  = "storagetier"
)

// node zone distribution
// nodes of old version types
// production AKS cluster without HA default node pool

func nodePremiumStorage(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	clusterNode := node.Object.(*corev1.Node)
	storageTier, ok := clusterNode.Labels[storageTierKey]
	if !ok {
		return false, nil, fmt.Errorf("label %s missing from node", storageTierKey)
	}
	if storageTier != "Premium_LRS" {
		return true, nil, nil
	}
	return false, nil, nil
}

func nodeBurstableTypes(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	clusterNode := node.Object.(*corev1.Node)
	instanceType, ok := clusterNode.Labels[instanceTypeKey]
	if !ok {
		return false, nil, fmt.Errorf("label %s missing from node", instanceTypeKey)
	}
	if strings.HasPrefix(instanceType, "Standard_B") {
		return true, nil, nil
	}
	return false, nil, nil
}

func nodeXKSLabels(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	clusterNode := node.Object.(*corev1.Node)
	if _, ok := clusterNode.Labels[classKey]; !ok {
		return true, nil, nil
	}
	if _, ok := clusterNode.Labels[purposeKey]; !ok {
		return true, nil, nil
	}
	return false, nil, nil
}

func nodeAKSDefaultNodePoolNoTaint(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	clusterNode := node.Object.(*corev1.Node)
	if _, ok := clusterNode.Labels[agentPoolKey]; !ok {
		return false, nil, nil
	}
	if clusterNode.Labels[agentPoolKey] != "default" {
		return false, nil, nil
	}
	for _, taint := range clusterNode.Spec.Taints {
		if taint.Key == "CriticalAddonsOnly" && taint.Effect == "NoSchedule" && taint.Value == "true" {
			return false, nil, nil
		}
	}
	return true, nil, nil
}
