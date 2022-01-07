package check

import (
	"context"
	iofs "io/fs"
	"strings"

	"github.com/xenitab/kube-checker/pkg/graph"
)

type Checker struct {
	rules        map[string][]Rule
	deprecations map[string]Deprecation
}

func NewChecker(fs iofs.FS) (*Checker, error) {
	deprecations, err := loadDeprecations(fs)
	if err != nil {
		return nil, err
	}
	rules := getRules()
	return &Checker{
		rules:        rules,
		deprecations: deprecations,
	}, nil
}

func (c *Checker) Evaluate(g *graph.Graph) (map[string]*RuleResult, error) {
	ruleResults := map[string]*RuleResult{}
	err := g.Iterate(func(node *graph.Node) error {
		// Check for api version deprecation
		ruleResult := deprecatedApiVersion(c.deprecations, node)
		if ruleResult != nil {
			if _, ok := ruleResults[ruleResult.Rule.ID]; !ok {
				ruleResults[ruleResult.Rule.ID] = ruleResult
			} else {
				ruleResults[ruleResult.Rule.ID].Violations = append(ruleResults[ruleResult.Rule.ID].Violations, ruleResult.Violations...)
			}
		}

		// Evaluate rules for each kind
		kindRules := c.rules[strings.ToLower(node.Reference.Kind)]
		kindRules = append(kindRules, c.rules["all"]...)
		for _, rule := range kindRules {
			hasViolated, messages, err := rule.Evaluate(context.Background(), node, g)
			if err != nil {
				return err
			}
			if !hasViolated {
				continue
			}
			rootNode := g.FindRootOwner(node)
			violation := Violation{
				Reference: rootNode.Reference,
				Message:   strings.Join(messages, ", "),
			}
			if _, ok := ruleResults[rule.ID]; !ok {
				ruleResults[rule.ID] = &RuleResult{
					Rule:       rule,
					Violations: []Violation{violation},
				}
				continue
			}
			ruleResults[rule.ID].AddViolation(violation)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ruleResults, nil
}

// cert manager annotations
// check that metrics are not exposed on ingress
// use of configmap and secrets without reloader configured
// pdb is set for all pods
// network policy ingress that allows everything on all applications
// pdb is used for multiple deployments
// secret that does not originate from anywhere
