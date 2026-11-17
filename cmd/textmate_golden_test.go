// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// TestTextMateGolden builds the TextMate grammars of several languages and
// compares them with the files in testdata/textmate/<case>. Run
// "go test ./cmd -run TestTextMateGolden -update" to rewrite them.
func TestTextMateGolden(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		languages []Language
		scopes    map[string]string
	}{
		{
			// Several files, two languages with declared per-language modes.
			name:  "multilang",
			input: "../internal/languages/multilang",
			languages: []Language{
				{Entry: "Greeting", TokenMode: "GreetingMode", LanguageID: "greeting", Patterns: []string{"*.hello"}},
				{Entry: "Farewell", TokenMode: "FarewellMode", LanguageID: "farewell", Patterns: []string{"*.bye"}},
			},
		},
		{
			// push, pop and nested modes.
			name:      "token_modes",
			input:     "../internal/languages/token_modes",
			languages: []Language{{Entry: "Model", LanguageID: "tokenmodes", Patterns: []string{"*.tokenmodes"}}},
		},
		{
			name:      "arithmetics",
			input:     "../examples/arithmetics",
			languages: []Language{{Entry: "Module", LanguageID: "arithmetics", Patterns: []string{"*.calc"}}},
			// SL_COMMENT is hidden, so it only gets a scope through an override.
			scopes: map[string]string{"SL_COMMENT": "comment.line"},
		},
		{
			name:      "statemachine",
			input:     "../examples/statemachine",
			languages: []Language{{Entry: "Statemachine", LanguageID: "statemachine", Patterns: []string{"*.statemachine"}}},
		},
		{
			// Strings with and without escapes, multi-line tokens and overrides.
			name:      "strings",
			input:     "testdata/textmate/input/strings",
			languages: []Language{{Entry: "Model", LanguageID: "strings", Patterns: []string{"*.str", "*.strings"}}},
			scopes:    map[string]string{"CHAR": "string.quoted.single", "ID": "variable.other"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := t.TempDir()
			ctx := &BuildContext{
				Input:          c.input,
				Output:         t.TempDir(),
				Package:        "golden",
				Languages:      c.languages,
				TextMateOutput: out,
				TextMateScopes: c.scopes,
			}
			require.NoError(t, ctx.Build())

			golden := filepath.Join("testdata", "textmate", c.name)
			generated := tmLanguageFiles(t, out)
			if *update {
				require.NoError(t, os.RemoveAll(golden))
				require.NoError(t, os.MkdirAll(golden, 0755))
				for _, name := range generated {
					content, err := os.ReadFile(filepath.Join(out, name))
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(filepath.Join(golden, name), content, 0644))
				}
				return
			}
			require.Equal(t, tmLanguageFiles(t, golden), generated, "generated files differ; run with -update")
			for _, name := range generated {
				want, err := os.ReadFile(filepath.Join(golden, name))
				require.NoError(t, err)
				got, err := os.ReadFile(filepath.Join(out, name))
				require.NoError(t, err)
				require.Equal(t, string(want), string(got), "%s differs; run with -update", name)
			}
		})
	}
}

// tmLanguageFiles returns the sorted names of the *.tmLanguage.json files in dir.
func tmLanguageFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.tmLanguage.json"))
	require.NoError(t, err)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, filepath.Base(match))
	}
	slices.Sort(names)
	return names
}
