package graph

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Graph struct {
	dg    *simple.DirectedGraph
	idMap map[string]int64
}

func NewGraph() *Graph {
	return &Graph{
		dg:    simple.NewDirectedGraph(),
		idMap: map[string]int64{},
	}
}

// Populate fills the graph with the contents of a cluster.
func (g *Graph) Populate(ctx context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, namespace string) error {
	logger := logr.FromContextOrDiscard(ctx).WithName("graph")
	logger.Info("discovering API resources")
	gvrs, err := discover(ctx, client, namespace != "")
	if err != nil {
		return fmt.Errorf("could not discover API resources: %w", err)
	}
  gvrs = filterGVRs(gvrs)
	logger.Info("fetching all resources")
	objects, err := fetch(ctx, dynamicClient, gvrs, namespace)
	if err != nil {
		return fmt.Errorf("could not fetch API resources: %w", err)
	}
	logger.Info("adding nodes")
	for _, u := range objects {
		err := g.AddUnstructuredNode(u)
		if err != nil {
			return err
		}
	}
	logger.Info("connecting edges")
	nodes := g.dg.Nodes()
	for {
		if !nodes.Next() {
			break
		}
		node := nodes.Node().(*Node)
		err := g.AddEdgesForNode(node)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddUnstructuredNode adds a node from a unstructured resource
func (g *Graph) AddUnstructuredNode(u unstructured.Unstructured) error {
  node, err := NewNode(u)
	if err != nil {
		return err
	}

	// Skip helm release secrets
  // TODO: Find a better place to check this
	if node.Reference.ApiVersion == "v1" && node.Reference.Kind == "Secret" && node.Object.(*corev1.Secret).Type == "helm.sh/release.v1" {
		return nil
	}

	g.dg.AddNode(node)
	g.idMap[node.Reference.ID()] = node.ID()
	return nil
}

// AddEdgesForNode adds all the edges for a specific node
func (g *Graph) AddEdgesForNode(node *Node) error {
	for _, ownerRef := range node.Unstructured.GetOwnerReferences() {
		if ownerRef.Controller == nil {
			continue
		}
		if !*ownerRef.Controller {
			continue
		}
		refId, err := uuid.Parse(string(ownerRef.UID))
		if err != nil {
      fmt.Println("owner ref", ownerRef.String())
			return err
		}
		refNode := g.dg.Node(int64(refId.ID()))
		if refNode == nil {
			return fmt.Errorf("to node cannot be nil")
		}
		edge := NewEdge(refNode, node, EdgeTypeOwner)
		g.dg.SetEdge(edge)
	}

	relationships := relationshipsForObject(node.Object)
	for _, relationship := range relationships {
		if relationship.Reference.Namespace == "" {
			relationship.Reference.Namespace = node.Reference.Namespace
		}
		id, ok := g.idMap[relationship.Reference.ID()]
		if !ok {
			return nil
      //return fmt.Errorf("missing reference: %s", relationship.Reference.ID())
		}
		refNode := g.dg.Node(id)

		var edge Edge
		switch relationship.Direction {
		case RelationshipDirectionFrom:
			edge = NewEdge(refNode, node, relationship.Type)
		case RelationshipDirectionTo:
			edge = NewEdge(node, refNode, relationship.Type)
		}
		g.dg.SetEdge(edge)
	}

	return nil
}

// TODO: Implement
func (g *Graph) List(gvk schema.GroupVersionKind) []Node {
	return []Node{}
}

// Edges returns a list of all edges to and from a node
func (g *Graph) Edges(node *Node) []Edge {
	edges := []Edge{}
	nodes := g.dg.To(node.ID())
	for {
		if !nodes.Next() {
			break
		}
		edge := g.dg.Edge(nodes.Node().ID(), node.ID())
		edges = append(edges, edge.(Edge))
	}
	nodes = g.dg.From(node.ID())
	for {
		if !nodes.Next() {
			break
		}
		edge := g.dg.Edge(node.ID(), nodes.Node().ID())
		edges = append(edges, edge.(Edge))
	}
	return edges
}

// Iterate lists all nodes in the graph
func (g *Graph) Iterate(f func(n *Node) error) error {
	nodes := g.dg.Nodes()
	for {
		if !nodes.Next() {
			break
		}
		node := nodes.Node().(*Node)
		err := f(node)
		if err != nil {
			return err
		}
	}
	return nil
}

// FindRootOwner returns the last node with a owner edge
func (g *Graph) FindRootOwner(n *Node) *Node {
	links := g.dg.To(n.id)
	if links.Len() == 0 {
		return n
	}
	for {
		if !links.Next() {
			break
		}
		edge := g.dg.Edge(links.Node().ID(), n.id).(Edge)
		if edge.Type != EdgeTypeOwner {
			continue
		}
		return g.FindRootOwner(links.Node().(*Node))
	}
	return n
}

// EncodeDot returns the graph in dot format
func (g *Graph) EncodeDot() ([]byte, error) {
	b, err := dot.Marshal(g.dg, "Kubernetes", "", "")
	if err != nil {
		return nil, err
	}
	return b, nil
}
