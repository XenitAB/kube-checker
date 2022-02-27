package check

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

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
	type lookupResult struct {
		results []net.IP
		err     error
	}
	lookupCache := make(map[string]lookupResult)
	lookupMutex := &sync.Mutex{}
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
		Net:     "tcp",
		Timeout: 1 * time.Second,
	}
	ns := net.JoinHostPort(conf.Servers[0], conf.Port)

	lookupFn := func(host string) ([]net.IP, error) {
		lookupMutex.Lock()
		defer lookupMutex.Unlock()

		v, ok := lookupCache[host]
		if ok {
			return v.results, v.err
		}

		msg := &dns.Msg{}
		msg.SetQuestion(fmt.Sprintf("%s.", host), dns.TypeA)

		result, _, err := dnsClient.Exchange(msg, ns)
		if err != nil {
			lookupCache[host] = lookupResult{results: nil, err: err}
			return nil, err
		}

		if result.Rcode != dns.RcodeSuccess {
			err := fmt.Errorf("response code not success for lookup of %s: %d", host, result.Rcode)
			lookupCache[host] = lookupResult{results: nil, err: err}
			return nil, err
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
			err := fmt.Errorf("received no results from lookup of %s", host)
			lookupCache[host] = lookupResult{results: nil, err: err}
			return nil, err
		}

		lookupCache[host] = lookupResult{results: results, err: nil}

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
