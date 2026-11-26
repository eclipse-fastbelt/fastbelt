// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLanguagesSingle(t *testing.T) {
	langs := Languages([]LanguageConfig{
		{LanguageID: "greeting", Entry: "Greeting", Patterns: []string{"*.hello", "*.hi"}},
	})
	require.Len(t, langs, 1)
	require.Equal(t, "greeting", langs[0].LanguageID)
	require.Equal(t, "Greeting", langs[0].Entry)
	require.Equal(t, []string{"*.hello", "*.hi"}, langs[0].Patterns)
}

func TestLanguagesMultipleKeepOrder(t *testing.T) {
	langs := Languages([]LanguageConfig{
		{LanguageID: "greeting", Entry: "Greeting", Patterns: []string{"*.hello"}},
		{LanguageID: "farewell", Entry: "Farewell", Patterns: []string{"*.bye"}},
		{LanguageID: "calc", Entry: "Calc", Patterns: []string{"*.calc"}},
	})
	require.Len(t, langs, 3)
	for i, want := range []struct{ id, entry, pattern string }{
		{"greeting", "Greeting", "*.hello"},
		{"farewell", "Farewell", "*.bye"},
		{"calc", "Calc", "*.calc"},
	} {
		require.Equal(t, want.id, langs[i].LanguageID)
		require.Equal(t, want.entry, langs[i].Entry)
		require.Equal(t, []string{want.pattern}, langs[i].Patterns)
	}
}

func TestLanguagesCopiesPatterns(t *testing.T) {
	config := []LanguageConfig{{LanguageID: "greeting", Entry: "Greeting", Patterns: []string{"*.hello"}}}
	langs := Languages(config)
	langs[0].Patterns[0] = "*.changed"
	langs[0].Patterns = append(langs[0].Patterns, "*.added")
	require.Equal(t, []string{"*.hello"}, config[0].Patterns)

	config[0].Patterns[0] = "*.other"
	require.Equal(t, "*.changed", langs[0].Patterns[0])
}

func TestLanguagesEmptyPatternsAreNil(t *testing.T) {
	langs := Languages([]LanguageConfig{
		{LanguageID: "a", Entry: "A"},
		{LanguageID: "b", Entry: "B", Patterns: []string{}},
	})
	require.Nil(t, langs[0].Patterns)
	require.Nil(t, langs[1].Patterns)
}

func TestLanguagesNone(t *testing.T) {
	require.Empty(t, Languages(nil))
}
