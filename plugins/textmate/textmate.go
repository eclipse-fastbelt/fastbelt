// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"maps"
	"strings"
)

// Options configures the TextMate plugin. See [New].
type Options struct {
	// CommonFile is the path of the common grammar shared by all languages.
	// It is required.
	CommonFile string
	// LanguageFiles maps language IDs to the path of their grammar. Languages
	// of the build without an entry get no grammar file.
	LanguageFiles map[string]string
	// Scopes maps token names to TextMate scopes, overriding the derived
	// scopes.
	Scopes map[string]string
}

// Plugin generates TextMate grammars. Create it with [New].
type Plugin struct {
	commonFile    string
	languageFiles map[string]string
	scopes        map[string]string
}

// New creates a TextMate plugin. The maps in options are copied, so later
// changes to them do not affect the plugin.
func New(options Options) *Plugin {
	return &Plugin{
		commonFile:    options.CommonFile,
		languageFiles: maps.Clone(options.LanguageFiles),
		scopes:        maps.Clone(options.Scopes),
	}
}

// Name returns "textmate".
func (p *Plugin) Name() string {
	return "textmate"
}

// LanguageScopeName returns the scope name of the grammar of a language:
// source.<languageID>.
func LanguageScopeName(languageID string) string {
	return "source." + languageID
}

// CommonScopeName returns the scope name of the common grammar of a
// Fastbelt grammar: source.<grammar>-common, with the grammar name in lower
// case.
func CommonScopeName(grammarName string) string {
	return "source." + lowerGrammarName(grammarName) + "-common"
}

// lowerGrammarName returns the grammar name as used in TextMate scope names:
// in lower case.
func lowerGrammarName(grammarName string) string {
	return strings.ToLower(grammarName)
}
