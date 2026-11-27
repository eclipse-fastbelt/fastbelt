// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

// Node is implemented by every node of the AST mirror. The concrete nodes are
// structs used as pointers, such as *[ParserRule]; abstract grammar types and
// unions are sealed interfaces, such as [AbstractRule].
type Node interface {
	// Container returns the node containing this one, or nil for the
	// [Grammar] root.
	Container() Node
	// Location returns the source range of the whole node in its original
	// .fb file. See the package documentation for nodes without a source.
	Location() Location
	isNode()
}

// node holds the state that every Node has. The converter sets it through
// the hooks in internal/artifacts/astinit.
type node struct {
	container Node
	location  Location
}

func (n *node) Container() Node    { return n.container }
func (n *node) Location() Location { return n.location }
func (*node) isNode()              {}

func (n *node) setContainer(c Node)    { n.container = c }
func (n *node) setLocation(l Location) { n.location = l }
