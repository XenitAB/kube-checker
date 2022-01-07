package check

/*import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

func seriveLabelSelectorMultipleApps(ctx context.Context, clientSet *kubernetes.Clientset) []error {
	svcList, err := clientSet.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	errs := []error{}
	for _, svc := range svcList.Items {
		// skip the special kuberentes service
		if svc.Namespace == "default" && svc.Name == "kubernetes" {
			continue
		}

		labelMap, err := metav1.LabelSelectorAsMap(&metav1.LabelSelector{MatchLabels: svc.Spec.Selector})
		opt := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()}
		podList, err := clientSet.CoreV1().Pods(svc.Namespace).List(ctx, opt)
		if err != nil {
			return []error{err}
		}
		// a service with no endpoints
		if len(podList.Items) == 0 {
			errs = append(errs, fmt.Errorf("service %s has no endpoints", svc.Name))
			continue
		}
		// no need to compare owners if there is only one pod
		if len(podList.Items) == 1 {
			continue
		}

		// make sure owner refs are the same for all pods
		ownerRefs := podList.Items[0].GetOwnerReferences()
		for _, pod := range podList.Items[1:] {
			if len(pod.GetOwnerReferences()) != len(ownerRefs) {
				errs = append(errs, fmt.Errorf("service %s target pods has mismatched owner length", svc.Name))
			}
			for i, owner := range pod.GetOwnerReferences() {
				cmpOwner := ownerRefs[i]
				if owner.APIVersion != cmpOwner.APIVersion && owner.Kind != cmpOwner.Kind && owner.Name != cmpOwner.Name {
					errs = append(errs, fmt.Errorf("service %s target pods has mismatched owners", svc.Name))
				}
			}
		}
	}

	if len(errs) != 0 {
		return errs
	}
	return nil
}*/
