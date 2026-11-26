// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/internal/languages/multilang"
	"typefox.dev/fastbelt/internal/languages/token_modes"
	"typefox.dev/fastbelt/pluginapi/tokens"
	"typefox.dev/fastbelt/test"
)

// modesResult holds the converted token types and modes of a grammar.
type modesResult struct {
	tts     []*tokens.TokenType
	modes   []*tokens.TokenMode
	entries []*tokens.TokenMode
}

// languageConfig configures one language for [tokenModesOf]: its entry rule and
// optional entry mode name.
type languageConfig struct{ entry, mode string }

// tokenModesOf converts the token modes of a grammar the way the build does.
func tokenModesOf(t *testing.T, text string, langs ...languageConfig) modesResult {
	t.Helper()
	f := test.New(t, grammar.CreateServices())
	doc := f.Parse(text).AssertNoErrors()
	g, ok := doc.Document.Root.(grammar.Grammar)
	require.True(t, ok)
	require.NoError(t, grammar.ExpandInfixRules(g))
	result := generator.GenerateTokenTypes(g)
	tts, err := TokenTypes(g, result)
	require.NoError(t, err)

	entryRules := make([]grammar.ParserRule, len(langs))
	entryModes := make([]string, len(langs))
	for i, lang := range langs {
		for _, rule := range g.Rules() {
			if rule.Name() == lang.entry {
				entryRules[i] = rule
			}
		}
		require.NotNil(t, entryRules[i], "entry rule %s", lang.entry)
		entryModes[i] = lang.mode
	}
	modes, entries, err := TokenModes(g, entryRules, entryModes, result, tts)
	require.NoError(t, err)
	requireModeInvariants(t, tts, modes)
	require.Len(t, entries, len(langs))
	for _, entry := range entries {
		require.Contains(t, modes, entry)
	}
	return modesResult{tts: tts, modes: modes, entries: entries}
}

// tokenModesOfFiles merges the grammar files into one text, keeping only
// the first grammar header.
func tokenModesOfFiles(t *testing.T, header string, paths []string, langs ...languageConfig) modesResult {
	t.Helper()
	text := ""
	for i, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		if i > 0 {
			// Only the first file keeps its grammar header.
			rest, ok := strings.CutPrefix(string(content), header)
			require.True(t, ok, "%s must start with %q", path, header)
			content = []byte(rest)
		}
		text += string(content) + "\n"
	}
	return tokenModesOf(t, text, langs...)
}

// requireModeInvariants asserts the index and linking invariants every mode
// list must hold.
func requireModeInvariants(t *testing.T, tts []*tokens.TokenType, modes []*tokens.TokenMode) {
	t.Helper()
	require.NotEmpty(t, modes)
	for i, mode := range modes {
		require.Equal(t, i, mode.Index, "mode %q", mode.Name)
		for _, member := range mode.Members {
			tt := member.TokenType
			require.Same(t, tts[tt.Index], tt, "mode %q", mode.Name)
			require.NotEqual(t, tokens.KindGroup, tt.Kind)
			require.NotEqual(t, tokens.KindEOF, tt.Kind)
			if cmd := member.Command; cmd != nil {
				if cmd.Kind == tokens.CommandPop {
					require.Nil(t, cmd.Target)
				} else {
					require.Same(t, modes[cmd.Target.Index], cmd.Target)
				}
			}
		}
	}
}

func modeByName(t *testing.T, modes []*tokens.TokenMode, name string) *tokens.TokenMode {
	t.Helper()
	for _, mode := range modes {
		if mode.Name == name {
			return mode
		}
	}
	require.Failf(t, "token mode not found", "%q", name)
	return nil
}

func modeNames(modes []*tokens.TokenMode) []string {
	result := make([]string, len(modes))
	for i, mode := range modes {
		result[i] = mode.Name
	}
	return result
}

func memberNames(mode *tokens.TokenMode) []string {
	result := make([]string, len(mode.Members))
	for i, member := range mode.Members {
		result[i] = member.TokenType.Name
	}
	return result
}

func modeMember(t *testing.T, mode *tokens.TokenMode, name string) *tokens.TokenModeMember {
	t.Helper()
	for _, m := range mode.Members {
		if m.TokenType.Name == name {
			return m
		}
	}
	require.Failf(t, "member not found", "%q in mode %q", name, mode.Name)
	return nil
}

func TestTokenModesWithoutDeclaredModesSingleLanguage(t *testing.T) {
	r := tokenModesOf(t, `
		grammar Test;
		interface Model { Name string }
		entry Model: Name=ID "end";
		interface Unused { Name string }
		Unused: "unused" Name=INT;
		token ID: /[a-z]+/;
		token INT: /[0-9]+/;
		hidden token WS: /\s+/;
		comment token SL_COMMENT: /\/\/[^\n]*/;
	`, languageConfig{entry: "Model"})

	require.Len(t, r.modes, 1)
	mode := r.modes[0]
	require.Equal(t, "default", mode.Name)
	require.False(t, mode.Synthetic)
	// A single language keeps all token types, reachable or not.
	require.Equal(t, []string{"end", "unused", "ID", "INT", "WS", "SL_COMMENT"}, memberNames(mode))
	require.Equal(t, tokens.ModifierNone, modeMember(t, mode, "ID").Modifier)
	require.Equal(t, tokens.ModifierHidden, modeMember(t, mode, "WS").Modifier)
	require.Equal(t, tokens.ModifierComment, modeMember(t, mode, "SL_COMMENT").Modifier)
	for _, m := range mode.Members {
		require.Nil(t, m.Command)
	}
	require.Same(t, mode, r.entries[0])
}

func TestTokenModesWithoutDeclaredModesSeveralLanguages(t *testing.T) {
	r := tokenModesOf(t, `
		grammar Test;
		interface Greeting { Name string }
		entry Greeting: "hello" Name=ID;
		interface Farewell { Count string }
		entry Farewell: "goodbye" Count=INT;
		token ID: /[a-z]+/;
		token INT: /[0-9]+/;
		hidden token WS: /\s+/;
		comment token SL_COMMENT: /\/\/[^\n]*/;
	`, languageConfig{entry: "Farewell"}, languageConfig{entry: "Greeting"})

	require.Equal(t, []string{"Farewell", "Greeting"}, modeNames(r.modes))
	for _, mode := range r.modes {
		require.True(t, mode.Synthetic, mode.Name)
	}
	// Only reachable token types, plus hidden and comment tokens.
	require.Equal(t, []string{"goodbye", "INT", "WS", "SL_COMMENT"}, memberNames(r.modes[0]))
	require.Equal(t, []string{"hello", "ID", "WS", "SL_COMMENT"}, memberNames(r.modes[1]))
	require.Equal(t, tokens.ModifierHidden, modeMember(t, r.modes[1], "WS").Modifier)
	require.Same(t, r.modes[0], r.entries[0])
	require.Same(t, r.modes[1], r.entries[1])
}

func TestTokenModesDeclaredModesMatchGeneratedLanguage(t *testing.T) {
	text, err := os.ReadFile("../languages/token_modes/token_modes.fb")
	require.NoError(t, err)
	r := tokenModesOf(t, string(text), languageConfig{entry: "Model"})

	require.Equal(t, []string{"default", "IN_STRING", "IN_INTERPOLATION"}, modeNames(r.modes))
	require.Equal(t, token_modes.TokenMode_default, modeByName(t, r.modes, "default").Index)
	require.Equal(t, token_modes.TokenMode_IN_STRING, modeByName(t, r.modes, "IN_STRING").Index)
	require.Equal(t, token_modes.TokenMode_IN_INTERPOLATION, modeByName(t, r.modes, "IN_INTERPOLATION").Index)
	for _, mode := range r.modes {
		require.False(t, mode.Synthetic, mode.Name)
	}
	def, inString, inInterpolation := r.modes[0], r.modes[1], r.modes[2]
	require.Same(t, def, r.entries[0])

	// Keywords first, in the order the mode lists them.
	require.Equal(t, []string{":=", "`", "+", "(", ")", "ID", "INT", "WS", "SL_COMMENT", "ML_COMMENT"}, memberNames(def))
	// STRING_CONTENT is declared before "#{", but keywords come first.
	require.Equal(t, []string{"`", "#{", "STRING_CONTENT"}, memberNames(inString))
	require.Equal(t, []string{"}", "`", "+", "(", ")", "ID", "INT", "WS", "SL_COMMENT", "ML_COMMENT"}, memberNames(inInterpolation))

	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandPush, Target: inString}, modeMember(t, def, "`").Command)
	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandPop}, modeMember(t, inString, "`").Command)
	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandPush, Target: inInterpolation}, modeMember(t, inString, "#{").Command)
	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandPop}, modeMember(t, inInterpolation, "}").Command)
	require.Nil(t, modeMember(t, def, "ID").Command)

	// The modes apply the modifiers; the declarations have none.
	require.Equal(t, tokens.ModifierNone, modeMember(t, def, "WS").TokenType.Modifier)
	require.Equal(t, tokens.ModifierHidden, modeMember(t, def, "WS").Modifier)
	require.Equal(t, tokens.ModifierComment, modeMember(t, inInterpolation, "SL_COMMENT").Modifier)
	require.Equal(t, tokens.ModifierNone, modeMember(t, inString, "STRING_CONTENT").Modifier)
}

func TestTokenModesDeclaredModesPerLanguageMatchGeneratedLanguage(t *testing.T) {
	r := tokenModesOfFiles(t, "grammar MultilangModel", []string{
		"../languages/multilang/multilang_common.fb",
		"../languages/multilang/multilang_farewell.fb",
		"../languages/multilang/multilang_greeting.fb",
	}, languageConfig{entry: "Greeting", mode: "GreetingMode"}, languageConfig{entry: "Farewell", mode: "FarewellMode"})

	// The implicit default mode is dropped.
	require.Equal(t, []string{"FarewellMode", "GreetingMode"}, modeNames(r.modes))
	require.Equal(t, multilang.TokenMode_FarewellMode, modeByName(t, r.modes, "FarewellMode").Index)
	require.Equal(t, multilang.TokenMode_GreetingMode, modeByName(t, r.modes, "GreetingMode").Index)
	require.Equal(t, "GreetingMode", r.entries[0].Name)
	require.Equal(t, "FarewellMode", r.entries[1].Name)
	require.Equal(t, []string{"hello", "ID", "WS"}, memberNames(r.entries[0]))
}

func TestTokenModesModeCommandAndModifierOverrides(t *testing.T) {
	r := tokenModesOf(t, `
		grammar Test;
		interface Model { Names []string }
		entry Model: "begin" Names+=ID* "end";
		interface Single { Name string }
		entry Single: Name=ID;
		token ID: /[a-z]+/;
		token WS: /\s+/;
		comment token SL_COMMENT: /\/\/[^\n]*/;

		token mode OUTER {
			"begin" -> mode(INNER)
			hidden WS
			SL_COMMENT
		}

		token mode INNER {
			ID
			"end" -> mode(OUTER)
			WS
			hidden SL_COMMENT
		}
	`, languageConfig{entry: "Model", mode: "OUTER"}, languageConfig{entry: "Single", mode: "INNER"})

	require.Equal(t, []string{"OUTER", "INNER"}, modeNames(r.modes))
	outer, inner := r.modes[0], r.modes[1]
	require.Same(t, outer, r.entries[0])
	require.Same(t, inner, r.entries[1])

	require.Equal(t, []string{"begin", "WS", "SL_COMMENT"}, memberNames(outer))
	require.Equal(t, []string{"end", "ID", "WS", "SL_COMMENT"}, memberNames(inner))
	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandMode, Target: inner}, modeMember(t, outer, "begin").Command)
	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandMode, Target: outer}, modeMember(t, inner, "end").Command)

	require.Equal(t, tokens.ModifierHidden, modeMember(t, outer, "WS").Modifier)
	require.Equal(t, tokens.ModifierNone, modeMember(t, inner, "WS").Modifier)
	// A plain usage keeps the modifier of the declaration, an explicit one overrides it.
	require.Equal(t, tokens.ModifierComment, modeMember(t, outer, "SL_COMMENT").Modifier)
	require.Equal(t, tokens.ModifierHidden, modeMember(t, inner, "SL_COMMENT").Modifier)
}

// A token declaration with keyword content is registered like a regexp token
// in a declared mode, even though its token type is a keyword.
func TestTokenModesKeywordDeclarationOrder(t *testing.T) {
	r := tokenModesOf(t, `
		grammar Test;
		interface Model { Name string }
		entry Model: BEGIN Name=ID "end";
		token BEGIN: "begin";
		token ID: /[a-z]+/;
		hidden token WS: /\s+/;

		token mode default {
			ID
			BEGIN
			"end"
			WS
		}
	`, languageConfig{entry: "Model"})

	require.Equal(t, []string{"end", "ID", "BEGIN", "WS"}, memberNames(r.modes[0]))
	require.Equal(t, tokens.KindKeyword, modeMember(t, r.modes[0], "BEGIN").TokenType.Kind)
}
