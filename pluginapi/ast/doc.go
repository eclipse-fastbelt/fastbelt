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
// [tokens.Modifier] and [tokens.CommandKind]. Cross-references point
// directly to the target node; an absent optional reference or child is
// nil. The source tokens themselves are not part of the mirror.
//
// [TokenDecl], [TokenGroup] and [Keyword] link to the token type they
// produce or match. For the reverse direction, see
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.TokenDeclaration].
//
// Rule bodies are not mirrored yet: Body fields are nil.
//
// # Locations
//
// [Node.Location] is the byte range of the whole node in the .fb file it was
// parsed from, also when several files were merged. The merged [Grammar] root
// of a multi-file grammar has no single file and returns a zero Location; the
// root of a single-file grammar covers the whole file.
//
// # Synthetic nodes
//
// Infix expansion adds an operator token group per infix rule to
// [Grammar.TokenGroups]. These nodes have Synthetic set ([TokenGroup.Synthetic],
// [Keyword.Synthetic]) and are located at the infix rule (the group) or the
// operator they were copied from (its keywords), in the file of the infix
// rule.
package ast
