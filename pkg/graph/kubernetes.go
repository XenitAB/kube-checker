package graph

import (
	"context"
	"fmt"

	"golang.org/x/sync/semaphore"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	//policyv1 "k8s.io/api/policy/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// discover returns all resources that are supported by the cluster.
func discover(ctx context.Context, client kubernetes.Interface, namespaced bool) ([]schema.GroupVersionResource, error) {
	logger := logr.FromContextOrDiscard(ctx).WithName("discover")
	groupList, err := client.Discovery().ServerPreferredResources()
	if err != nil {
		return nil, err
	}
	storageVersionHash := map[string]bool{}
	gvrs := []schema.GroupVersionResource{}
	for _, group := range groupList {
		if len(group.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(group.GroupVersion)
		if err != nil {
			return nil, fmt.Errorf("could not parse group version: %w", err)
		}
		for _, res := range group.APIResources {
			// TODO: Need a better way of detecting same kind but different group versions
			if _, ok := storageVersionHash[res.StorageVersionHash]; ok {
				logger.V(1).Info("skipping duplicate resource", "group", gv.Group, "version", gv.Version, "kind", res.Kind)
				continue
			}
			storageVersionHash[res.StorageVersionHash] = true
			// Skip resources which cant be listed
			if !allowsList(res.Verbs) {
				logger.V(1).Info("skipping non listable resource", "group", gv.Group, "version", gv.Version, "kind", res.Kind)
				continue
			}
			// Skip cluster wide resources if only discovering namespaced
			if namespaced && !res.Namespaced {
				logger.V(1).Info("skipping cluster wide resource", "group", gv.Group, "version", gv.Version, "kind", res.Kind)
				continue
			}
			gvr := gv.WithResource(res.Name)
			gvrs = append(gvrs, gvr)
		}
	}
	return gvrs, err
}

func allowsList(verbs metav1.Verbs) bool {
	for _, verb := range verbs {
		if verb == "list" {
			return true
		}
	}
	return false
}

// fetch returns all objects of all resources in the cluster.
func fetch(ctx context.Context, client dynamic.Interface, gvrs []schema.GroupVersionResource, namespace string) ([]unstructured.Unstructured, error) {
	sem := semaphore.NewWeighted(int64(10))
	resultCh := make(chan []unstructured.Unstructured)
	errorCh := make(chan error)
	for _, gvr := range gvrs {
		sem.Acquire(ctx, int64(1))
		go func(gvr schema.GroupVersionResource) {
			var list *unstructured.UnstructuredList
			var err error
			if namespace == "" {
				list, err = client.Resource(gvr).List(ctx, metav1.ListOptions{})
			} else {
				list, err = client.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
			}
			if err != nil && !apierrors.IsNotFound(err) {
				errorCh <- fmt.Errorf("could not list gvr %s: %w", gvr.String(), err)
				return
			}
			sem.Release(int64(1))
			resultCh <- list.Items
		}(gvr)
	}

	resultCounter := 0
	objects := []unstructured.Unstructured{}
	for {
		select {
		case items := <-resultCh:
			for _, item := range items {
				objects = append(objects, item)
			}
		case err := <-errorCh:
			return nil, err
		}
		resultCounter++
		if len(gvrs) == resultCounter {
			break
		}
	}
	return objects, nil
}

func parseRuntimeObject(u unstructured.Unstructured) (runtime.Object, error) {
	switch u.GetKind() {
	case "Node":
		node := &corev1.Node{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, node)
		if err != nil {
			return nil, err
		}
		return node, nil
	case "Secret":
		secret := &corev1.Secret{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, secret)
		if err != nil {
			return nil, err
		}
		return secret, nil
	case "Pod":
		pod := &corev1.Pod{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pod)
		if err != nil {
			return nil, err
		}
		return pod, nil
	case "DaemonSet":
		ds := &appsv1.DaemonSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, ds)
		if err != nil {
			return nil, err
		}
		return ds, nil
	case "Ingress":
		ingress := &networkingv1.Ingress{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, ingress)
		if err != nil {
			return nil, err
		}
		return ingress, nil
	case "ServiceAccount":
		sa := &corev1.ServiceAccount{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sa)
		if err != nil {
			return nil, err
		}
		return sa, nil
	case "RoleBinding":
		rb := &rbacv1.RoleBinding{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, rb)
		if err != nil {
			return nil, err
		}
		return rb, nil
	case "HorizontalPodAutoscaler":
		hpa := &autoscalingv1.HorizontalPodAutoscaler{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, hpa)
		if err != nil {
			return nil, err
		}
		return hpa, nil
	case "EndpointSlice":
		slice := &discoveryv1.EndpointSlice{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, slice)
		if err != nil {
			return nil, err
		}
		return slice, nil
	case "Kustomization":
		kustomization := &kustomizev1.Kustomization{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, kustomization)
		if err != nil {
			return nil, err
		}
		return kustomization, nil
	case "GitRepository":
		repo := &sourcev1.GitRepository{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, repo)
		if err != nil {
			return nil, err
		}
		return repo, nil
	default:
		return nil, nil
	}
}

func relationshipsForObject(object runtime.Object) []RelationshipDescription {
	relationships := []RelationshipDescription{}
	switch object.(type) {
	case *corev1.Pod:
		pod := object.(*corev1.Pod)
		if aadPodId, ok := pod.Labels["aadpodidbinding"]; ok {
			relationship := RelationshipDescription{
				Type:      EdgeTypeConsumes,
				Direction: RelationshipDirectionFrom,
				Reference: ObjectReference{
					ApiVersion: "aadpodidentity.k8s.io/v1",
					Kind:       "AzureIdentityBinding",
					Namespace:  "",
					Name:       aadPodId,
				},
			}
			relationships = append(relationships, relationship)
		}
		relationships = append(relationships, RelationshipDescription{
			Type:      EdgeTypeConsumes,
			Direction: RelationshipDirectionTo,
			Reference: ObjectReference{
				ApiVersion: "v1",
				Kind:       "ServiceAccount",
				Namespace:  "",
				Name:       pod.Spec.ServiceAccountName,
			},
		})
		for _, volume := range pod.Spec.Volumes {
			if volume.Secret != nil {
				relationships = append(relationships, RelationshipDescription{
					Type:      EdgeTypeConsumes,
					Direction: RelationshipDirectionTo,
					Reference: ObjectReference{
						ApiVersion: "v1",
						Kind:       "Secret",
						Namespace:  "",
						Name:       volume.Secret.SecretName,
					},
				})
			}
			if volume.ConfigMap != nil {
				relationships = append(relationships, RelationshipDescription{
					Type:      EdgeTypeConsumes,
					Direction: RelationshipDirectionTo,
					Reference: ObjectReference{
						ApiVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "",
						Name:       volume.ConfigMap.Name,
					},
				})
			}
		}
	case *discoveryv1.EndpointSlice:
		endpointSlice := object.(*discoveryv1.EndpointSlice)
		for _, endpoint := range endpointSlice.Endpoints {
			relationships = append(relationships, RelationshipDescription{
				Type:      EdgeTypeReference,
				Direction: RelationshipDirectionTo,
				Reference: ObjectReference{
					ApiVersion: "v1",
					Kind:       endpoint.TargetRef.Kind,
					Namespace:  endpoint.TargetRef.Namespace,
					Name:       endpoint.TargetRef.Name,
				},
			})
		}
	case *corev1.Secret:
		secret := object.(*corev1.Secret)
		if certName, ok := secret.Annotations["cert-manager.io/certificate-name"]; ok {
			relationship := RelationshipDescription{
				Type:      EdgeTypeOwner,
				Direction: RelationshipDirectionFrom,
				Reference: ObjectReference{
					ApiVersion: "cert-manager.io/v1",
					Kind:       "Certificate",
					Namespace:  "",
					Name:       certName,
				},
			}
			relationships = append(relationships, relationship)
		}
	case *corev1.ServiceAccount:
		sa := object.(*corev1.ServiceAccount)
		for _, secret := range sa.Secrets {
			relationship := RelationshipDescription{
				Type:      EdgeTypeOwner,
				Direction: RelationshipDirectionTo,
				Reference: ObjectReference{
					ApiVersion: "v1",
					Kind:       "Secret",
					Namespace:  "",
					Name:       secret.Name,
				},
			}
			relationships = append(relationships, relationship)
		}
	case *networkingv1.Ingress:
		ingress := object.(*networkingv1.Ingress)
		for _, rule := range ingress.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				relationship := RelationshipDescription{
					Type:      EdgeTypeReference,
					Direction: RelationshipDirectionTo,
					Reference: ObjectReference{
						ApiVersion: "v1",
						Kind:       "Service",
						Namespace:  "",
						Name:       path.Backend.Service.Name,
					},
				}
				relationships = append(relationships, relationship)
			}
		}
	/*case *policyv1.PodDisruptionBudget:
	pdb := object.(*policyv1.PodDisruptionBudget)*/
	case *rbacv1.RoleBinding:
		rb := object.(*rbacv1.RoleBinding)
		relationships = append(relationships, RelationshipDescription{
			Type:      EdgeTypeReference,
			Direction: RelationshipDirectionTo,
			Reference: ObjectReference{
				ApiVersion: "rbac.authorization.k8s.io/v1",
				Kind:       rb.RoleRef.Kind,
				Namespace:  "",
				Name:       rb.RoleRef.Name,
			},
		})
		for _, subject := range rb.Subjects {
			if subject.Kind != "ServiceAccount" {
				continue
			}
			relationships = append(relationships, RelationshipDescription{
				Type:      EdgeTypeReference,
				Direction: RelationshipDirectionTo,
				Reference: ObjectReference{
					ApiVersion: "v1",
					Kind:       subject.Kind,
					Namespace:  subject.Namespace,
					Name:       subject.Name,
				},
			})
		}
	case *autoscalingv1.HorizontalPodAutoscaler:
		hpa := object.(*autoscalingv1.HorizontalPodAutoscaler)
		relationship := RelationshipDescription{
			Type:      EdgeTypeReference,
			Direction: RelationshipDirectionTo,
			Reference: ObjectReference{
				ApiVersion: hpa.Spec.ScaleTargetRef.APIVersion,
				Kind:       hpa.Spec.ScaleTargetRef.Kind,
				Namespace:  "",
				Name:       hpa.Spec.ScaleTargetRef.Name,
			},
		}
		relationships = append(relationships, relationship)
	case *kustomizev1.Kustomization:
		kust := object.(*kustomizev1.Kustomization)
		relationships = append(relationships, RelationshipDescription{
			Type:      EdgeTypeConsumes,
			Direction: RelationshipDirectionTo,
			Reference: ObjectReference{
				ApiVersion: "v1",
				Kind:       "ServiceAccount",
				Namespace:  "",
				Name:       kust.Spec.ServiceAccountName,
			},
		})
		relationships = append(relationships, RelationshipDescription{
			Type:      EdgeTypeConsumes,
			Direction: RelationshipDirectionTo,
			Reference: ObjectReference{
				ApiVersion: "source.toolkit.fluxcd.io/v1beta1",
				Kind:       kust.Spec.SourceRef.Kind,
				Namespace:  "",
				Name:       kust.Spec.SourceRef.Name,
			},
		})
		for _, ref := range kust.Spec.HealthChecks {
			relationships = append(relationships, RelationshipDescription{
				Type:      EdgeTypeReference,
				Direction: RelationshipDirectionTo,
				Reference: ObjectReference{
					ApiVersion: ref.APIVersion,
					Kind:       ref.Kind,
					Namespace:  ref.Namespace,
					Name:       ref.Name,
				},
			})
		}
	case *sourcev1.GitRepository:
		repo := object.(*sourcev1.GitRepository)
		relationships = append(relationships, RelationshipDescription{
			Type:      EdgeTypeConsumes,
			Direction: RelationshipDirectionTo,
			Reference: ObjectReference{
				ApiVersion: "v1",
				Kind:       "Secret",
				Namespace:  "",
				Name:       repo.Spec.SecretRef.Name,
			},
		})
	}
	return relationships
}
