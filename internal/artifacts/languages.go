// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"slices"

	"typefox.dev/fastbelt/pluginapi"
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
