package check

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/xenitab/kube-checker/pkg/graph"
)

// use of disallowed nginx annotations or other custom features

func ingressNoClass(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	ingress := node.Object.(*networkingv1.Ingress)
	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "" {
		return false, nil, nil
	}
	return true, nil, nil
}

func ingressNoTLS(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	ingress := node.Object.(*networkingv1.Ingress)
	if len(ingress.Spec.TLS) > 0 {
		return false, nil, nil
	}
	return true, nil, nil
}
