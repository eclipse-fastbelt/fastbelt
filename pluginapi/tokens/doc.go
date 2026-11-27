// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package tokens describes the token types and token modes of a generated
// Fastbelt lexer, as handed to plugins via
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts].
//
// It is the bottom layer of the plugin API and imports no other artifact
// package; the AST mirror in [typefox.dev/fastbelt/pluginapi/ast] links down
// to it and shares its [Modifier] and [CommandKind] constants.
//
// # Token types
//
// A [TokenType] stands for one runtime token index of the generated lexer:
// BuildArtifacts.TokenTypes[i].Index == i, and the index equals the ID of the
// token type at runtime (core.TokenType.Id and the generated *_Idx constant).
// The indices are dense, with EOF at 0, then all keywords, all regexp tokens
// and all token groups. A [TokenType] describes what the lexer matches
// ([TokenType.Keyword], [TokenType.Regexp], or the [TokenType.Members] of a
// group) but carries no automata.
//
// # Token modes
//
// A [TokenMode] is one of the effective modes the generated lexer runs, which
// can differ from the modes declared in the grammar (see [TokenMode] for the
// rules). Its members are listed in lexer priority order, each with the
// [Modifier] and the [TokenCommand] effective in that mode. A command links
// to its target mode, so the modes form a graph that may contain cycles.
//
// The entry mode of each language is
// [typefox.dev/fastbelt/pluginapi.Language.EntryMode].
package tokens
