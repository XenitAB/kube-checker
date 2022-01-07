package check

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/xenitab/kube-checker/pkg/graph"
)

const (
	kustomizeNameKey      = "kustomize.toolkit.fluxcd.io/name"
	kustomizeNamespaceKey = "kustomize.toolkit.fluxcd.io/namespace"
	helmNamespaceKey      = "helm.toolkit.fluxcd.io/namespace"
	helmNameKey           = "helm.toolkit.fluxcd.io/name"
)

func fluxUnmanagedResource(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	logger := logr.FromContextOrDiscard(ctx).WithName("flux")

	// Get the root owner resource
	node = graph.FindRootOwner(node)

	// Ignore certain resources that are automatically created by Kubernetes
	ignoreResources := map[string]bool{
		fmt.Sprintf("v1/ServiceAccount/%s/default", node.Reference.Namespace):     true,
		fmt.Sprintf("v1/ConfigMap/%s/kube-root-ca.crt", node.Reference.Namespace): true,
	}
	if _, ok := ignoreResources[node.Reference.ID()]; ok {
		logger.Info("ignore Kubernetes created resource", "key", node.Reference.ID())
		return false, nil, nil
	}

	// Check if resource is managed by a Kustomization or HelmRelease
	labels := node.Unstructured.GetLabels()
	if labels[kustomizeNamespaceKey] != "" && labels[kustomizeNameKey] != "" {
		return false, nil, nil
	}
	if labels[helmNamespaceKey] != "" && labels[helmNameKey] != "" {
		return false, nil, nil
	}
	return true, nil, nil
}
