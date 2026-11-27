// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package astinit lets the converter in [typefox.dev/fastbelt/internal/artifacts]
// and the build in [typefox.dev/fastbelt/cmd] set the unexported state of
// [typefox.dev/fastbelt/pluginapi/ast] nodes and of
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts].
//
// It holds only func variables with any parameters and imports nothing, so
// that pluginapi/ast and pluginapi can import it without a cycle. They fill
// the variables in their init functions; the converter and the build call
// them. Being internal, the package is not reachable for plugin authors.
package astinit

// SetContainer sets the parent node of an ast node. Both arguments are ast
// nodes (pointers to the concrete node structs); container is nil for the
// root. Set by pluginapi/ast.
var SetContainer func(node, container any)

// SetLocation sets the location of an ast node. node is an ast node (a
// pointer to a concrete node struct), location an ast.Location value. Set by
// pluginapi/ast.
var SetLocation func(node, location any)

// SetTokenDeclarations sets the map behind
// pluginapi.BuildArtifacts.TokenDeclaration. artifacts is a
// *pluginapi.BuildArtifacts, declarations a
// map[*tokens.TokenType]ast.TokenDeclaration. Set by pluginapi.
var SetTokenDeclarations func(artifacts, declarations any)
