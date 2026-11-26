// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package pluginapi

import (
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// Plugin produces additional output from the artifacts of a Fastbelt build.
// See the package documentation for when and how plugins are called.
type Plugin interface {
	// Name identifies the plugin in error messages.
	Name() string
	// Build runs the plugin. The artifacts are shared with other plugins and
	// must not be modified.
	Build(artifacts BuildArtifacts) error
}

// BuildArtifacts holds everything a Fastbelt build produced that plugins may
// use. It is read-only by contract.
type BuildArtifacts struct {
	// InputDir is the absolute path of the directory containing the .fb grammar files.
	InputDir string
	// OutputDir is the absolute path of the directory the generated Go files
	// were written to. Plugins resolve relative output paths against it.
	OutputDir string
	// Package is the Go package name of the generated code.
	Package string
	// Languages are the languages configured for the build, in configured
	// order. There is at least one.
	Languages []*Language
	// TokenTypes are the token types of the generated lexer, indexed by their
	// runtime token index: TokenTypes[i].Index == i. Index 0 is EOF.
	TokenTypes []*tokens.TokenType
	// TokenModes are the token modes the generated lexer runs, indexed by
	// their generated mode constant: TokenModes[i].Index == i. See
	// [tokens.TokenMode] for how they relate to the declared modes. There is
	// at least one.
	TokenModes []*tokens.TokenMode
	// Sources are the .fb grammar files of the build, sorted by path.
	Sources []*ast.SourceFile
}
