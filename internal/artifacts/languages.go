// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"fmt"
	"slices"

	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/ast"
)

// LanguageConfig holds the configured values of one language of the build. It
// mirrors the fields of cmd.Language that make up a [pluginapi.Language], so
// this package does not have to import cmd.
type LanguageConfig struct {
	LanguageID string
	Entry      string
	Patterns   []string
}

// Languages converts the configured languages into plugin artifacts, keeping
// their order. Patterns are copied; empty patterns become nil.
func Languages(configs []LanguageConfig) []*pluginapi.Language {
	langs := make([]*pluginapi.Language, len(configs))
	for i, config := range configs {
		var patterns []string
		if len(config.Patterns) > 0 {
			patterns = slices.Clone(config.Patterns)
		}
		langs[i] = &pluginapi.Language{
			LanguageID: config.LanguageID,
			Entry:      config.Entry,
			Patterns:   patterns,
		}
	}
	return langs
}

// EntryRules returns the mirrors of the entry parser rules of the languages,
// index-aligned to entries. g and root are the grammar and its mirror, see
// [Grammar]; every entry must be one of g.Rules().
func EntryRules(g grammar.Grammar, root *ast.Grammar, entries []grammar.ParserRule) ([]*ast.ParserRule, error) {
	rules := make([]*ast.ParserRule, len(entries))
	for i, entry := range entries {
		// root.Rules is converted from g.Rules() in order.
		index := slices.Index(g.Rules(), entry)
		if index < 0 || index >= len(root.Rules) {
			return nil, fmt.Errorf("entry rule %s is not a rule of the grammar", entry.Name())
		}
		rules[i] = root.Rules[index]
	}
	return rules, nil
}
