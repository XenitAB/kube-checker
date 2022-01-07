package check

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenitab/kube-checker/pkg/graph"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestApiVersionDeprecatedBasic(t *testing.T) {
	node := &graph.Node{
		Namespace:        "bar",
		Name:             "baz",
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Foo"},
		ManagedFields: []v1.ManagedFieldsEntry{
			{
				APIVersion: "v1beta1",
			},
		},
	}
	deprecations := []deprecation{
		{
			ApiVersion:    "v1beta1",
			Kind:          "Foo",
			NewApiVersion: "v1",
		},
	}
	ruleResult := apiVersionDeprecated(deprecations, node)
	require.NotNil(t, ruleResult)
	require.Equal(t, "APIVersionDeprecated-v1/Foo", ruleResult.Rule.ID)
	require.Equal(t, uint(10), ruleResult.Rule.Severity)
	require.NotEmpty(t, ruleResult.Violations)
	require.Equal(t, "bar", ruleResult.Violations[0].Namespace)
	require.Equal(t, "baz", ruleResult.Violations[0].Name)
}
