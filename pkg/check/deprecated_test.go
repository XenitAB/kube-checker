package check

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenitab/kube-checker/pkg/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeprecatedApiVersionBasic(t *testing.T) {
	node := &graph.Node{
		Unstructured: unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Foo",
				"metadata": map[string]interface{}{
					"managedFields": []interface{}{
						map[string]interface{}{
							"apiVersion": "v1beta1",
						},
					},
				}}},
		Object: nil,
		Reference: graph.ObjectReference{
			ApiVersion: "v1",
			Kind:       "Foo",
			Namespace:  "bar",
			Name:       "baz",
		},
	}
	deprecations := map[string]Deprecation{
		"v1beta1/Foo": {
			ApiVersion:    "v1beta1",
			Kind:          "Foo",
			NewApiVersion: "v1",
		},
	}
	ruleResult := deprecatedApiVersion(deprecations, node)
	require.NotNil(t, ruleResult)
	require.Equal(t, "APIVersionDeprecated/v1/Foo", ruleResult.Rule.ID)
	require.Equal(t, uint(10), ruleResult.Rule.Severity)
	require.NotEmpty(t, ruleResult.Violations)
	require.Equal(t, "bar", ruleResult.Violations[0].Reference.Namespace)
	require.Equal(t, "baz", ruleResult.Violations[0].Reference.Name)
}
