package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/olekukonko/tablewriter"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/xenitab/kube-checker/pkg/check"
	"github.com/xenitab/kube-checker/pkg/graph"
)

//go:embed deprecated-versions.yaml
var fs embed.FS

func main() {
	// Get flag inputs
	namespace := flag.String("namespace", "", "The namespace to scope to.")
	kubeconfigPath := flag.String("kubeconfig", "", "Path to the kubeconfig file.")
	flag.Parse()

	// Discard any logs from the Kubernetes client
	klog.SetLogger(logr.Discard())
	// Setup logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	logger := zapr.NewLogger(zapLog)

	// Run application
	if err := run(logger, *namespace, *kubeconfigPath); err != nil {
		logger.Error(err, "error running application")
		os.Exit(1)
	}
}

func run(logger logr.Logger, namespace, kubeconfigPath string) error {
	// Setup context
	ctx := context.Background()
	ctx = logr.NewContext(ctx, logger)

	// Get cluster clients
	client, dynamicClient, err := getKubernetesClients(kubeconfigPath)
	if err != nil {
		return err
	}

	// Check the cluster resources
	g := graph.NewGraph()
	err = g.Populate(ctx, client, dynamicClient, namespace)
	if err != nil {
		return err
	}
	checker, err := check.NewChecker(fs)
	if err != nil {
		return err
	}
	ruleResults, err := checker.Evaluate(g)
	if err != nil {
		return err
	}

	b, err := g.EncodeDot()
	if err != nil {
		return err
	}
	os.WriteFile("/home/philip/graph.dot", b, 0777)

	// Print result
	checkTable := tablewriter.NewWriter(os.Stdout)
	checkTable.SetHeader([]string{"ID", "Severity", "Description"})
	violationTable := tablewriter.NewWriter(os.Stdout)
	violationTable.SetHeader([]string{"Api Version", "Kind", "Namespace", "Name", "Message"})
	for _, r := range ruleResults {
		if len(r.Violations) == 0 {
			continue
		}

		fmt.Printf("\n\n\n\n")

		checkTable.ClearRows()
		checkTable.Append([]string{r.Rule.ID, strconv.FormatUint(uint64(r.Rule.Severity), 10), r.Rule.Description})
		checkTable.Render()

		violationTable.ClearRows()
		for _, v := range r.Violations {
			violationTable.Append([]string{v.Reference.ApiVersion, v.Reference.Kind, v.Reference.Namespace, v.Reference.Name, v.Message})
		}
		violationTable.Render()
	}
	return nil
}

func getKubernetesClients(path string) (kubernetes.Interface, dynamic.Interface, error) {
	cfg, err := getKubernetesConfig(path)
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, dynamicClient, nil
}

func getKubernetesConfig(path string) (*rest.Config, error) {
	if path != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", path)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
