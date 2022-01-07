package graph

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ObjectReference struct {
	ApiVersion string
	Kind       string
	Namespace  string
	Name       string
}

func (o ObjectReference) ID() string {
	return strings.Join([]string{o.ApiVersion, o.Kind, o.Namespace, o.Name}, "/")
}

func (o ObjectReference) GVK() string {
	return strings.Join([]string{o.ApiVersion, o.Kind}, "/")
}

type Node struct {
	id           int64
	Unstructured unstructured.Unstructured
	Object       runtime.Object
	Reference    ObjectReference
}

func NewNode(u unstructured.Unstructured) (*Node, error) {
	reference := ObjectReference{
		ApiVersion: u.GetAPIVersion(),
		Kind:       u.GetKind(),
		Namespace:  u.GetNamespace(),
		Name:       u.GetName(),
	}

	object, err := parseRuntimeObject(u)
	if err != nil {
		return nil, err
	}
	if string(u.GetUID()) == "" {
		return nil, fmt.Errorf("resource uuid is empty: %s", reference.ID())
	}
	id, err := uuid.Parse(string(u.GetUID()))
	if err != nil {
		return nil, err
	}

	node := &Node{
		id:           int64(id.ID()),
		Unstructured: u,
		Object:       object,
		Reference:    reference,
	}
	return node, nil
}

func (n *Node) ID() int64 {
	return n.id
}

func (n *Node) DOTID() string {
	return n.Reference.ID()
}

func (n *Node) ResourceID() string {
	return n.Reference.ID()
}
