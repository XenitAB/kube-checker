package check

import (
	"context"
	"fmt"
	"net"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/miekg/dns"
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

func ingressDNS() func(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		conf = &dns.ClientConfig{
			Servers: []string{
				"1.1.1.1",
			},
			Port: "53",
		}
	}
	dnsClient := &dns.Client{
		Net: "tcp",
	}
	dnsConn, err := dnsClient.Dial(net.JoinHostPort(conf.Servers[0], conf.Port))
	if err != nil {
		return func(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
			return true, []string{fmt.Sprintf("unable to connect to dns: %v", err)}, nil
		}
	}

	lookupFn := func(host string) ([]net.IP, error) {
		msg := &dns.Msg{}
		queryHost := host
		if !strings.HasSuffix(host, ".") {
			queryHost = fmt.Sprintf("%s.", host)
		}
		msg.SetQuestion(queryHost, dns.TypeA)

		result, _, err := dnsClient.ExchangeWithConn(msg, dnsConn)
		if err != nil {
			return nil, err
		}

		if result.Rcode != dns.RcodeSuccess {
			return nil, fmt.Errorf("response code not success for lookup of %s: %d", host, result.Rcode)
		}

		results := []net.IP{}
		for _, ip := range result.Answer {
			record, ok := ip.(*dns.A)
			if ok {
				if record.A.To4() == nil {
					continue
				}
				results = append(results, record.A)
			}
		}

		if len(results) == 0 {
			return nil, fmt.Errorf("received no results from lookup of %s", host)
		}

		return results, nil
	}

	return func(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
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
				results, err := lookupFn(host)
				if err != nil {
					return true, []string{fmt.Sprintf("unable to lookup ip for %q: %v", host, err)}, nil
				}

				if results[0].String() != ip {
					return true, []string{fmt.Sprintf("lookup of %q expected ip %q but received: %s", host, ip, results[0].String())}, nil
				}
			}
		}

		return false, nil, nil
	}
}
