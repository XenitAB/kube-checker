package main

import (
	"context"
	"embed"
	"encoding/gob"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alexflint/go-arg"
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
	"github.com/xenitab/kube-checker/pkg/tui"
)

//go:embed deprecated-versions.yaml
var fs embed.FS

func main() {
	// Load config
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "config generation returned an error: %v\n", err)
		os.Exit(1)
	}

	// Discard any logs from the Kubernetes client
	klog.SetLogger(logr.Discard())
	// Setup logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger creation returned an error: %v\n", err)
		os.Exit(1)
	}
	logger := zapr.NewLogger(zapLog)
	ctx := logr.NewContext(context.Background(), logger)

	// Run application
	if err := run(ctx, cfg); err != nil {
		logger.Error(err, "error running application")
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config) error {
	var ruleResults map[string]*check.RuleResult
	var err error

	if cfg.ReadBinaryPath == "" {
		ruleResults, err = getResults(ctx, cfg)
		if err != nil {
			return err
		}

		if cfg.DumpBinaryPath != "" {
			err := dumpBinary(cfg.DumpBinaryPath, ruleResults)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "successfully dumped results to: %s\n", cfg.DumpBinaryPath)
			return nil
		}
	} else {
		ruleResults, err = readBinary(cfg.ReadBinaryPath)
		if err != nil {
			return err
		}
	}

	// Print result
	switch getOutputFormat(cfg.OutputFormat) {
	case tableOutputFormat:
		outputTable(ruleResults)
	case tuiOutputFormat:
		return outputTui(ruleResults)
	case unknownOutputFormat:
		return fmt.Errorf("unknown output format: %s (supported are: table, tui)", cfg.OutputFormat)
	}
	return nil
}

func dumpBinary(filePath string, v map[string]*check.RuleResult) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gob.Register(map[string]*check.RuleResult{})
	enc := gob.NewEncoder(f)

	return enc.Encode(&v)
}

func readBinary(filePath string) (map[string]*check.RuleResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)

	var v map[string]*check.RuleResult
	err = dec.Decode(&v)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func getResults(ctx context.Context, cfg config) (map[string]*check.RuleResult, error) {
	// Get cluster clients
	client, dynamicClient, err := getKubernetesClients(cfg.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	// Check the cluster resources
	g := graph.NewGraph()
	err = g.Populate(ctx, client, dynamicClient, cfg.Namespace)
	if err != nil {
		return nil, err
	}
	checker, err := check.NewChecker(fs)
	if err != nil {
		return nil, err
	}

	ruleResults, err := checker.Evaluate(g)
	if err != nil {
		return nil, err
	}

	if cfg.GraphFile != "" {
		b, err := g.EncodeDot()
		if err != nil {
			return nil, err
		}

		err = os.WriteFile(cfg.GraphFile, b, 0644)
		if err != nil {
			return nil, err
		}
	}

	return ruleResults, nil
}

func outputTable(ruleResults map[string]*check.RuleResult) {
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
}

func outputTui(ruleResults map[string]*check.RuleResult) error {
	return tui.Run(ruleResults)
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

type config struct {
	Namespace      string `arg:"--namespace,env:NAMESPACE" help:"the namespace to scope to"`
	KubeConfigPath string `arg:"--kubeconfig,env:KUBE_CONFIG" help:"path to the kubeconfig file"`
	GraphFile      string `arg:"--graph-file,env:GRAPH_FILE" help:"path to the stored graph file"`
	OutputFormat   string `arg:"--output-format,env:OUTPUT_FORMAT" default:"table" help:"what output format to use [table|tui]"`
	DumpBinaryPath string `arg:"--dump-binary-path,env:DUMP_BINARY_PATH" help:"set to the path where the results should be dumped"`
	ReadBinaryPath string `arg:"--read-binary-path,env:READ_BINARY_PATH" help:"from where should the results be read"`
}

func loadConfig(args []string) (config, error) {
	argCfg := arg.Config{
		Program:   "kube-checker",
		IgnoreEnv: false,
	}

	var cfg config
	parser, err := arg.NewParser(argCfg, &cfg)
	if err != nil {
		return config{}, err
	}

	err = parser.Parse(args)
	if err != nil {
		return config{}, err
	}

	return cfg, nil
}

type outputFormat int

const (
	unknownOutputFormat = iota + 1
	tableOutputFormat
	tuiOutputFormat
)

func getOutputFormat(s string) outputFormat {
	switch strings.ToLower(s) {
	case "table":
		return tableOutputFormat
	case "tui":
		return tuiOutputFormat
	}

	return unknownOutputFormat
}
