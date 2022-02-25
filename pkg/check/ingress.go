package check

import (
	"context"
	"fmt"
	"net"

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

func ingressDNS(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	ingress := node.Object.(*networkingv1.Ingress)

	hostExists := len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != ""
	lbIpExists := len(ingress.Status.LoadBalancer.Ingress) > 0 && ingress.Status.LoadBalancer.Ingress[0].IP != ""

	if hostExists && lbIpExists {
		if len(ingress.Status.LoadBalancer.Ingress) > 1 {
			return true, []string{"lookups of multiple ip addresses not implemented"}, nil
		}

		ip := ingress.Status.LoadBalancer.Ingress[0].IP

		for _, ingressRule := range ingress.Spec.Rules {
			host := ingressRule.Host
			results, err := net.LookupIP(host)
			if err != nil {
				return true, []string{fmt.Sprintf("unable to lookup ip for %q: %v", host, err)}, nil
			}

			var ipv4Result []net.IP
			for _, result := range results {
				if result.To4() == nil {
					continue
				}
				ipv4Result = append(ipv4Result, result)
			}

			if len(ipv4Result) != 1 {
				return true, []string{fmt.Sprintf("lookup of %q returned more than one ipv4 ip: %v", host, ipv4Result)}, nil
			}

			if ipv4Result[0].String() != ip {
				return true, []string{fmt.Sprintf("lookup of %q expected ip %q but received: %s", host, ip, ipv4Result[0].String())}, nil
			}
		}
	}

	return false, nil, nil
}
