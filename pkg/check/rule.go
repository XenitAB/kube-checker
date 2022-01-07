package check

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/xenitab/kube-checker/pkg/graph"
)

type EvaluateFunction func(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error)

type Rule struct {
	ID          string
	Severity    uint
	Description string
	Link        string
	Evaluate    EvaluateFunction
}

func (r *Rule) Validate() error {
	if len(r.ID) == 0 {
		return errors.New("id cannot be empty")
	}
	if r.Severity == 0 || r.Severity > 10 {
		return errors.New("severity cannot be 0 or greater than 10")
	}
	if len(r.Description) == 0 {
		return errors.New("message cannot be empty")
	}
	if len(r.Link) == 0 {
		return errors.New("link cannot be empty")
	}
	if _, err := url.Parse(r.Link); err != nil {
		return fmt.Errorf("link has to be a parseable url: %w", err)
	}
	return nil
}

type Violation struct {
	Reference graph.ObjectReference
	Message   string
}

type RuleResult struct {
	Rule       Rule
	Violations []Violation
}

func (r *RuleResult) AddViolation(violation Violation) {
	for _, v := range r.Violations {
		if v.Reference.ID() == violation.Reference.ID() {
			return
		}
	}
	r.Violations = append(r.Violations, violation)
}

func getRules() map[string][]Rule {
	return map[string][]Rule{
		"all": {
			{
				ID:          "MissingManagedFields",
				Severity:    0,
				Description: "Resource does not have managed fields set.",
				Link:        "",
				Evaluate:    missingManagedFields,
			},
			{
				ID:          "MixedApiVersions",
				Severity:    8,
				Description: "Resource is using different API Versions.",
				Link:        "",
				Evaluate:    mixedApiVersions,
			},
			{
				ID:          "UnusedResource",
				Severity:    6,
				Description: "Resources is not used.",
				Link:        "",
				Evaluate:    unusedResource,
			},
			{
				ID:          "FluxUnmanagedResource",
				Severity:    8,
				Description: "Resources are not managed by Flux.",
				Link:        "",
				Evaluate:    fluxUnmanagedResource,
			},
		},
		"node": {
			{
				ID:          "PremiumStorage",
				Severity:    5,
				Description: "Node should use premium storage.",
				Link:        "",
				Evaluate:    nodePremiumStorage,
			},
			{
				ID:          "BurstableInstanceType",
				Severity:    5,
				Description: "Node should not use burstable types.",
				Link:        "",
				Evaluate:    nodeBurstableTypes,
			},
			{
				ID:          "XKSLabel",
				Severity:    3,
				Description: "Node missing XKS labels.",
				Link:        "",
				Evaluate:    nodeXKSLabels,
			},
			{
				ID:          "AKSDefaultNodePoolNoTaint",
				Severity:    3,
				Description: "AKS default node pool is missing taint.",
				Link:        "",
				Evaluate:    nodeAKSDefaultNodePoolNoTaint,
			},
		},
		"pod": {
			{
				ID:          "WithoutController",
				Severity:    8,
				Description: "Pods should not be created without a controller.",
				Link:        "",
				Evaluate:    podWithoutController,
			},
			{
				ID:          "AutomountServiceAccountToken",
				Severity:    1,
				Description: "Pod should not automount service account tokens.",
				Link:        "",
				Evaluate:    podDoNotAutomountServiceAccount,
			},
			{
				ID:          "ImagePullPolicyAlways",
				Severity:    1,
				Description: "Pod is using image pull policy always.",
				Link:        "",
				Evaluate:    podImagePullPolicyAlways,
			},
			{
				ID:          "ReadinessInitialDelayHigh",
				Severity:    3,
				Description: "Readiness probe initial delay is high, did you mean to use startup probe?",
				Link:        "",
				Evaluate:    podReadinessInitialDelayHigh,
			},
			{
				ID:          "MissingReadinessProbe",
				Severity:    5,
				Description: "Pod missing readiness probe.",
				Link:        "",
				Evaluate:    podReadinessInitialDelayHigh,
			},
			{
				ID:          "ReadinessAndLivenessSame",
				Severity:    5,
				Description: "A pods readiness and liveness probe is the same.",
				Link:        "",
				Evaluate:    podReadinessAndLivenessSame,
			},
		},
		"daemonset": {
			{
				ID:          "OnAllNodes",
				Severity:    5,
				Description: "Daemonset is not running on all nodes.",
				Link:        "",
				Evaluate:    daemonsetOnAllNodes,
			},
		},
		"ingress": {
			{
				ID:          "NoClass",
				Severity:    3,
				Description: "Ingress is missing a class.",
				Link:        "",
				Evaluate:    ingressNoClass,
			},
			{
				ID:          "NoTLS",
				Severity:    6,
				Description: "Ingress is missing TLS configuration.",
				Link:        "",
				Evaluate:    ingressNoTLS,
			},
		},
	}
}
