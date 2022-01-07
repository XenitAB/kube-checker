package check

import (
	"context"

	"github.com/xenitab/kube-checker/pkg/graph"
)

// find use of non stored crd version

func mixedApiVersions(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	if len(node.Unstructured.GetManagedFields()) == 0 {
		return false, nil, nil
	}
	apiVersions := map[string]bool{}
	for _, mf := range node.Unstructured.GetManagedFields() {
		apiVersions[mf.APIVersion] = true
	}
	if len(apiVersions) == 1 {
		return false, nil, nil
	}
	messages := []string{}
	for k := range apiVersions {
		messages = append(messages, k)
	}
	return true, messages, nil
}

func missingManagedFields(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	return len(node.Unstructured.GetManagedFields()) == 0, nil, nil
}

func unusedResource(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	edges := graph.Edges(node)
	if len(edges) > 0 {
		return false, nil, nil
	}
	return true, nil, nil
}
