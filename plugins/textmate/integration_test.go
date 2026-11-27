// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"typefox.dev/fastbelt/cmd"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/plugins/textmate"
)

const integrationGrammar = `
	grammar Multi
	interface Greeting { Name string }
	interface Farewell { Name string }
	entry Greeting: "hello" Name=ID
	entry Farewell: "goodbye" Name=ID
	token ID: /[_a-zA-Z][\w_]*/
	hidden token WS: /\s+/
	token mode default {
		"hello" -> push(Inner)
		ID
		hidden WS
	}
	token mode Inner {
		"goodbye"
		ID -> pop
		hidden WS
	}
`

// collectingHandler collects the messages of all log records.
type collectingHandler struct {
	slog.Handler
	messages []string
}

func (h *collectingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *collectingHandler) Handle(_ context.Context, r slog.Record) error {
	h.messages = append(h.messages, r.Message)
	return nil
}

func TestBuildContextWithPlugin(t *testing.T) {
	input := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(input, "multi.fb"), []byte(integrationGrammar), 0o644))
	out := t.TempDir()
	absoluteFarewell := filepath.Join(t.TempDir(), "elsewhere", "farewell.tmLanguage.json")
	handler := &collectingHandler{Handler: slog.DiscardHandler}

	ctx := &cmd.BuildContext{
		Input:   input,
		Output:  out,
		Package: "multi",
		Languages: []cmd.Language{
			{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
			{Entry: "Farewell", TokenMode: "Inner", LanguageID: "farewell", Patterns: []string{"**/*.bye"}},
		},
		Logger: slog.New(handler),
		Plugins: []pluginapi.Plugin{textmate.New(textmate.Options{
			CommonFile: "syntaxes/multi-common.tmLanguage.json",
			LanguageFiles: map[string]string{
				"greeting": "syntaxes/greeting.tmLanguage.json",
				"farewell": absoluteFarewell,
			},
			Scopes: map[string]string{"ID": "variable.other", "NOPE": "comment"},
		})},
	}
	require.NoError(t, ctx.Build())

	require.Equal(t, []string{"TextMate: scope override for unknown token NOPE"}, handler.messages)

	commonScope := "source.multi-common"
	require.Equal(t, commonScope, textmate.CommonScopeName("Multi"))
	common := readJSON(t, filepath.Join(out, "syntaxes", "multi-common.tmLanguage.json"))
	require.Equal(t, commonScope, common["scopeName"])
	repository := common["repository"].(map[string]any)
	require.Contains(t, repository, "Greeting")
	require.Contains(t, repository, "Farewell")
	commonJSON, err := json.Marshal(common)
	require.NoError(t, err)
	require.Contains(t, string(commonJSON), `"variable.other.multi"`)

	for _, c := range []struct{ path, languageID, scope, entry string }{
		{filepath.Join(out, "syntaxes", "greeting.tmLanguage.json"), "greeting", "source.greeting", "Greeting"},
		{absoluteFarewell, "farewell", "source.farewell", "Farewell"},
	} {
		language := readJSON(t, c.path)
		require.Equal(t, c.scope, textmate.LanguageScopeName(c.languageID))
		require.Equal(t, c.scope, language["scopeName"])
		require.Equal(t, []any{map[string]any{"include": commonScope + "#" + c.entry}}, language["patterns"])
	}
}

// readJSON decodes the JSON file at path into a map.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))
	return result
}
