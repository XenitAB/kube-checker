package graph

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
)

type EdgeType string

const (
	EdgeTypeOwner         EdgeType = "owner"
	EdgeTypeConsumes      EdgeType = "consumes"
	EdgeTypeReference     EdgeType = "reference"
	EdgeTypeLabelSelector EdgeType = "label selector"
)

func (et EdgeType) Color() string {
	switch et {
	case EdgeTypeOwner:
		return "green"
	case EdgeTypeConsumes:
		return "red"
	case EdgeTypeReference:
		return "blue"
	case EdgeTypeLabelSelector:
		return "yellow"
	default:
		return "black"
	}
}

type Edge struct {
	Type EdgeType
	F, T graph.Node
}

func (e Edge) Attributes() []encoding.Attribute {
	return []encoding.Attribute{
		{
			Key:   "label",
			Value: string(e.Type),
		},
		{
			Key:   "color",
			Value: e.Type.Color(),
		},
	}
}

func NewEdge(from, to graph.Node, et EdgeType) Edge {
	return Edge{
		Type: et,
		F:    from,
		T:    to,
	}
}

func (e Edge) From() graph.Node {
	return e.F
}

func (e Edge) To() graph.Node {
	return e.T
}

func (e Edge) ReversedEdge() graph.Edge {
	return NewEdge(e.T, e.F, e.Type)
}

type RelationshipDirection string

const (
	RelationshipDirectionTo   RelationshipDirection = "to"
	RelationshipDirectionFrom RelationshipDirection = "from"
)

type RelationshipDescription struct {
	Reference ObjectReference
	Type      EdgeType
	Direction RelationshipDirection
}
