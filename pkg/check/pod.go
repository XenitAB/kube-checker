package check

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/xenitab/kube-checker/pkg/graph"
)

// basic pod security settings
// pod without priority class
// no resource requests or limits
// memory request is the same as memory limit
// cpu limit is set
// pod anti afinitiy
// pods with node selectors that can never be fullfilled
// affinity is no longer fullfilled
// pods from same deployment running on the same node
// toleration for spot but not running on spot instance
// use of ${} instead of $() for environment variables
// no disk resource limit set
// very large docker images'
// pods running as root user

func podWithoutController(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	pod := node.Object.(*corev1.Pod)
	return len(pod.GetOwnerReferences()) == 0, nil, nil
}

func podDoNotAutomountServiceAccount(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	pod := node.Object.(*corev1.Pod)
	if pod.Spec.AutomountServiceAccountToken != nil && *pod.Spec.AutomountServiceAccountToken == false {
		return false, nil, nil
	}
	return true, nil, nil
}

func podImagePullPolicyAlways(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	pod := node.Object.(*corev1.Pod)
	messages := []string{}
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, c := range containers {
		if c.ImagePullPolicy != corev1.PullAlways {
			continue
		}
		messages = append(messages, fmt.Sprintf("container %s", c.Name))
	}
	return len(messages) > 0, messages, nil
}

func podReadinessInitialDelayHigh(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	pod := node.Object.(*corev1.Pod)
	messages := []string{}
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, c := range containers {
		if c.ReadinessProbe == nil {
			continue
		}
		if c.ReadinessProbe.InitialDelaySeconds <= 30 {
			continue
		}
		messages = append(messages, fmt.Sprintf("container %s", c.Name))
	}
	return len(messages) > 0, messages, nil
}

func podMissingReadinessProbe(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	pod := node.Object.(*corev1.Pod)
	messages := []string{}
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, c := range containers {
		if c.ReadinessProbe != nil {
			continue
		}
		messages = append(messages, fmt.Sprintf("container %s", c.Name))
	}
	return len(messages) > 0, messages, nil
}

func podReadinessAndLivenessSame(ctx context.Context, node *graph.Node, graph *graph.Graph) (bool, []string, error) {
	pod := node.Object.(*corev1.Pod)
	messages := []string{}
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, c := range containers {
		if c.ReadinessProbe == nil || c.LivenessProbe == nil {
			continue
		}
		if c.ReadinessProbe.HTTPGet == nil || c.LivenessProbe.HTTPGet == nil {
			continue
		}
		if c.ReadinessProbe.HTTPGet.Path != c.LivenessProbe.HTTPGet.Path && c.ReadinessProbe.HTTPGet.Port != c.LivenessProbe.HTTPGet.Port {
			continue
		}
		messages = append(messages, fmt.Sprintf("container %s.", c.Name))
	}
	return len(messages) > 0, messages, nil
}
