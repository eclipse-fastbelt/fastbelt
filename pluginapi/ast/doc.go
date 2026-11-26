// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package ast is a read-only mirror of the Fastbelt grammar AST, as handed to
// plugins via [typefox.dev/fastbelt/pluginapi.BuildArtifacts].
//
// Nodes may link down to [typefox.dev/fastbelt/pluginapi/tokens], but never up
// to [typefox.dev/fastbelt/pluginapi/types] or [typefox.dev/fastbelt/pluginapi].
//
// # Structure
//
// The mirror covers the merged grammar of all .fb files of a build, after
// the infix rules have been expanded. The types carry the names of the
// grammar language's interfaces (see grammar/doc.go): a concrete node such as
// [ParserRule] is a struct used as a pointer, and an abstract type or union
// such as [AbstractRule] or [FieldType] is a sealed interface, implemented
// only by the node types of this package. Every node implements [Node].
//
// Values are plain Go values rather than source tokens: names are strings,
// keywords are unquoted ([Keyword.Value]), regexps have no surrounding
// slashes ([RegexpTokenContent.Regexp]), flags such as [ParserRule.Entry]
// are bools, and fixed choices are typed constants such as [Primitive],
// [Cardinality], [AssignmentOperator], [ActionOperator], [Associativity],
// [tokens.Modifier] and [tokens.CommandKind]. Cross-references point
// directly to the target node; an absent optional reference or child is
// nil. The source tokens themselves are not part of the mirror.
//
// [TokenDecl], [TokenGroup], [Keyword] and [RuleCall] (to a token
// declaration or group) link to the token type they produce or consume; a
// [CrossRef] consumes the token of its [CrossRef.Rule]. For the reverse
// direction, see
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.TokenDeclaration]; there is
// no list of the usages of a token type.
//
// [Node.Container] returns the parent node, and nil for the [Grammar] root.
// Together with the cross-references, this makes the AST a graph with
// cycles; walk the child fields of a node, not its container, to traverse it.
//
// # Locations
//
// [Node.Location] is the byte range of the whole node in the .fb file it was
// parsed from, also when several files were merged. The merged [Grammar] root
// of a multi-file grammar has no single file and returns a zero Location; the
// root of a single-file grammar covers the whole file.
//
// Offsets are bytes into [SourceFile.Text]. [SourceFile.LineColumn] converts
// an offset into a 1-based line and a 1-based column counted in runes, as
// usually shown to users.
//
// # Synthetic nodes
//
// Infix expansion adds an operator token group per infix rule to
// [Grammar.TokenGroups] and sets the [InfixRule.Body]. These nodes have
// Synthetic set ([TokenGroup.Synthetic], [Keyword.Synthetic],
// [Group.Synthetic], [RuleCall.Synthetic]) and are located in the file of
// the infix rule: at the operator they were copied from (the keywords of the
// group), at the operand call (the operand calls of the body), or at the
// infix rule itself (all others).
package ast
