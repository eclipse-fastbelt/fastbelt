// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package pluginapi

import (
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// Language is one of the languages configured for the build, see
// [BuildArtifacts.Languages].
type Language struct {
	// LanguageID is the ID used for LSP documents of this language. It is
	// unique among the languages of a build.
	LanguageID string
	// Entry is the name of the parser rule the language is entered with, as
	// configured. It is unique among the languages of a build.
	Entry string
	// Patterns are the glob patterns matching documents of this language on
	// the file system, in configured order. The slice is a copy of the
	// configuration. It is nil if no patterns are configured.
	Patterns []string
	// EntryMode is the token mode the lexer starts in for documents of this
	// language. It is one of [BuildArtifacts.TokenModes].
	EntryMode *tokens.TokenMode
	// EntryRule is the parser rule named by Entry. It is one of
	// [BuildArtifacts.Grammar]'s Rules.
	EntryRule *ast.ParserRule
}
