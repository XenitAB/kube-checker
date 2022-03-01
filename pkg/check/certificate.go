package check

import (
	"context"
	"fmt"
	"time"

	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	"github.com/xenitab/kube-checker/pkg/graph"
	"k8s.io/apimachinery/pkg/runtime"
)

func certificateExpiry(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	cert := &certv1.Certificate{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(node.Unstructured.Object, cert)
	if err != nil {
		return true, []string{fmt.Sprintf("unable to convert unstructured to *certv1.Certificate: %v", err)}, nil
	}
	if time.Now().Add(5 * 24 * time.Hour).After(cert.Status.NotAfter.Time) {
		return true, []string{fmt.Sprintf("certificate has expired or is about to: %s", cert.Status.NotAfter.Time.String())}, nil
	}

	return false, nil, nil
}
