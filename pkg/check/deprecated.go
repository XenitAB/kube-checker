package check

import (
	"fmt"
	iofs "io/fs"
	"strings"

	"github.com/xenitab/kube-checker/pkg/graph"
	"gopkg.in/yaml.v2"
)

type Deprecation struct {
	Component string `yaml:"component"`

	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`

	DeprecatedIn  string `yaml:"deprecatedIn"`
	RemovedIn     string `yaml:"removedIn"`
	NewApiVersion string `yaml:"newApiVersion"`

	Description string `yaml:"description"`
	Link        string `yaml:"link"`
}

func loadDeprecations(fs iofs.FS) (map[string]Deprecation, error) {
	b, err := iofs.ReadFile(fs, "deprecated-versions.yaml")
	if err != nil {
		return nil, fmt.Errorf("could not read deprecated versions file: %w", err)
	}
	deprecations := &[]Deprecation{}
	err = yaml.Unmarshal(b, deprecations)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal deprecated versions file: %w", err)
	}

	deprecationMap := map[string]Deprecation{}
	for _, deprecation := range *deprecations {
		if deprecation.Link == "" {
			return nil, fmt.Errorf("link cannot be empty")
		}
		if deprecation.Kind == "" {
			return nil, fmt.Errorf("kind cannot be empty")
		}
		if deprecation.ApiVersion == deprecation.NewApiVersion {
			return nil, fmt.Errorf("deprecated api version %s and new apiversion %s cannot be the same", deprecation.ApiVersion, deprecation.NewApiVersion)
		}
		key := strings.Join([]string{deprecation.ApiVersion, deprecation.Kind}, "/")
		if _, ok := deprecationMap[key]; ok {
			return nil, fmt.Errorf("duplicate key found: %s", key)
		}
		deprecationMap[key] = deprecation
	}
	return deprecationMap, nil
}

func deprecatedApiVersion(deprecations map[string]Deprecation, node *graph.Node) *RuleResult {
	for _, mf := range node.Unstructured.GetManagedFields() {
		key := strings.Join([]string{mf.APIVersion, node.Reference.Kind}, "/")
		deprecation, ok := deprecations[key]
		if !ok {
			continue
		}
		return &RuleResult{
			Rule: Rule{
				ID:          fmt.Sprintf("APIVersionDeprecated/%s", node.Reference.GVK()),
				Severity:    10,
				Description: fmt.Sprintf("Api version %q has been deprecated since version %s and will be removed in %s, please switch to %q", deprecation.ApiVersion, deprecation.DeprecatedIn, deprecation.RemovedIn, deprecation.NewApiVersion),
				Link:        deprecation.Link,
			},
			Violations: []Violation{
				{
					Reference: node.Reference,
				},
			},
		}
	}
	return nil
}
