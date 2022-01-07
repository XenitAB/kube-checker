package check

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/xenitab/kube-checker/pkg/graph"
)

func daemonsetOnAllNodes(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	ds := node.Object.(*appsv1.DaemonSet)
	nodeCount := len(graph.List(schema.GroupVersionKind{Version: "v1", Kind: "Node"}))
	if ds.Status.DesiredNumberScheduled == int32(nodeCount) {
		return false, nil, nil
	}
	return true, nil, nil
}
