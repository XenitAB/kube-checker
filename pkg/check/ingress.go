package check

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"

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

func ingressExternalDnsTxtOwner() func(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
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

	lookupFn := func(host string) ([]string, error) {
		msg := &dns.Msg{}
		queryHost := host
		if !strings.HasSuffix(host, ".") {
			queryHost = fmt.Sprintf("%s.", host)
		}
		msg.SetQuestion(queryHost, dns.TypeTXT)

		result, _, err := dnsClient.ExchangeWithConn(msg, dnsConn)
		if err != nil {
			return nil, err
		}

		if result.Rcode != dns.RcodeSuccess {
			return nil, fmt.Errorf("response code not success for lookup of %s: %d", host, result.Rcode)
		}

		results := []string{}
		for _, ip := range result.Answer {
			record, ok := ip.(*dns.TXT)
			if ok {
				results = append(results, record.Txt...)
			}
		}

		if len(results) == 0 {
			return nil, fmt.Errorf("received no results from lookup of %s", host)
		}

		return results, nil
	}

	mu := &sync.Mutex{}
	var externalDnsTxtOwner *string
	var externalDnsTxtOwnerErr error
	toStringPtr := func(s string) *string { return &s }
	getExternalDnsTxtOwner := func(g *graph.Graph) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		if externalDnsTxtOwner != nil {
			return *externalDnsTxtOwner, externalDnsTxtOwnerErr
		}

		var externalDns *appsv1.Deployment
		err := g.Iterate(func(node *graph.Node) error {
			if externalDns != nil {
				return nil
			}

			if node.Reference.Namespace != "external-dns" || node.Reference.Name != "external-dns" || node.Reference.Kind != "Deployment" {
				return nil
			}

			d := &appsv1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(node.Unstructured.Object, d)
			if err != nil {
				return err
			}

			externalDns = d

			return nil
		})

		if err != nil {
			externalDnsTxtOwner = toStringPtr("")
			externalDnsTxtOwnerErr = fmt.Errorf("unable to locate external-dns deployment: %v", err)
			return *externalDnsTxtOwner, externalDnsTxtOwnerErr
		}

		if len(externalDns.Spec.Template.Spec.Containers) != 1 {
			externalDnsTxtOwner = toStringPtr("")
			externalDnsTxtOwnerErr = fmt.Errorf("expected external-dns to have 1 container, received: %d", len(externalDns.Spec.Template.Spec.Containers))
			return *externalDnsTxtOwner, externalDnsTxtOwnerErr
		}

		container := externalDns.Spec.Template.Spec.Containers[0]
		for _, arg := range container.Args {
			if strings.HasPrefix(arg, "--txt-owner-id=") {
				externalDnsTxtOwner = toStringPtr(strings.TrimPrefix(arg, "--txt-owner-id="))
				return *externalDnsTxtOwner, nil
			}
		}

		externalDnsTxtOwner = toStringPtr("")
		externalDnsTxtOwnerErr = fmt.Errorf("unable to locate --txt-owner-id in args: %s", container.Args)
		return *externalDnsTxtOwner, externalDnsTxtOwnerErr
	}

	return func(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
		ingress := node.Object.(*networkingv1.Ingress)

		hostExists := len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != ""
		expectedOwner, err := getExternalDnsTxtOwner(graph)
		if err != nil {
			return true, []string{fmt.Sprintf("unable to get txt owner from external-dns pod: %v", err)}, nil
		}

		if hostExists {
			for _, ingressRule := range ingress.Spec.Rules {
				host := ingressRule.Host
				results, err := lookupFn(host)
				if err != nil {
					return true, []string{fmt.Sprintf("unable to lookup txt record for %q: %v", host, err)}, nil
				}

				for _, r := range results {
					if strings.Contains(r, "external-dns/owner=") {
						comp := strings.Split(r, ",")
						for _, c := range comp {
							if strings.HasPrefix(c, "external-dns/owner=") {
								lookupOwner := strings.TrimPrefix(c, "external-dns/owner=")
								if lookupOwner == expectedOwner {
									return false, nil, nil
								}
								return true, []string{fmt.Sprintf("expected external dns owner %q but found %q", expectedOwner, lookupOwner)}, nil
							}
						}
					}
				}
			}
		}

		return false, nil, nil
	}
}
