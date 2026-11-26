// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package astinit lets the converter in [typefox.dev/fastbelt/internal/artifacts]
// set the unexported state of [typefox.dev/fastbelt/pluginapi/ast] nodes.
//
// It holds only func variables with any parameters and imports nothing, so
// that pluginapi/ast can import it without a cycle. pluginapi/ast fills the
// variables in its init function; the converter calls them. Being internal,
// the package is not reachable for plugin authors.
package astinit

// SetContainer sets the parent node of an ast node. Both arguments are ast
// nodes (pointers to the concrete node structs); container is nil for the
// root. Nil until pluginapi/ast defines its node types.
var SetContainer func(node, container any)

// SetLocation sets the location of an ast node. node is an ast node (a
// pointer to a concrete node struct), location an ast.Location value. Set by
// pluginapi/ast.
var SetLocation func(node, location any)
