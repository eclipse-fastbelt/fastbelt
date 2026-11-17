// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/test"
)

// textMateModelFor parses src and builds the TextMate model. If entries is
// true, all entry rules of the grammar are passed as languages.
func textMateModelFor(t *testing.T, src string, entries bool, entryModes []string) textMateModel {
	t.Helper()
	f := test.New(t, grammar.CreateServices())
	doc := f.Parse(src)
	grammr, ok := doc.Document.Root.(grammar.Grammar)
	require.True(t, ok)
	var entryRules []grammar.ParserRule
	if entries {
		for _, rule := range grammr.Rules() {
			if rule.IsEntry() {
				entryRules = append(entryRules, rule)
			}
		}
	}
	return buildTextMateModel(grammr, entryRules, entryModes, GenerateTokenTypes(grammr))
}

// summarize renders the tokens of a mode as "Name[:modifier][ ->command(target)]".
func summarize(mode *textMateMode) []string {
	var result []string
	for _, token := range mode.Tokens {
		s := token.Name
		if token.Modifier != "" {
			s += ":" + token.Modifier
		}
		if token.Command != "" {
			s += " ->" + token.Command
			if token.Target != "" {
				s += "(" + token.Target + ")"
			}
		}
		result = append(result, s)
	}
	return result
}

func modeNames(model textMateModel) []string {
	var result []string
	for _, mode := range model.Modes {
		result = append(result, mode.Name)
	}
	return result
}

func TestTextMateModelImplicitDefaultMode(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: "hello" Name=ID ";";
		token ID: /[a-z]+/;
		hidden token WS: /\s+/;
		comment token SL_COMMENT: /\/\/[^\n\r]*/;
	`, true, nil)

	assert.Equal(t, []string{"default"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{{Entry: "Foo", EntryMode: "default"}}, model.Languages)
	// Keywords come first, like in the lexer.
	assert.Equal(t, []string{"hello", "Semicolon", "ID", "WS:hidden", "SL_COMMENT:comment"}, summarize(model.Modes[0]))

	tokens := model.Modes[0].Tokens
	assert.Equal(t, "hello", tokens[0].Keyword)
	assert.Equal(t, "", tokens[0].Regexp)
	assert.Equal(t, `[a-z]+`, tokens[2].Regexp)
	assert.Equal(t, "", tokens[2].Keyword)
	assert.Equal(t, `\/\/[^\n\r]*`, tokens[4].Regexp)
}

func TestTextMateModelWithoutEntryRules(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Foo { Name string }
		Foo: Name=ID;
		token ID: /[a-z]+/;
	`, false, nil)

	assert.Equal(t, []textMateLanguage{{EntryMode: "default"}}, model.Languages)
}

func TestTextMateModelKeywordToken(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: THEN Name=ID;
		token THEN: "then";
		token ID: /[a-z]+/;
	`, true, nil)

	tokens := model.Modes[0].Tokens
	require.NotEmpty(t, tokens)
	var then *textMateToken
	for i := range tokens {
		if tokens[i].Name == "THEN" {
			then = &tokens[i]
		}
	}
	require.NotNil(t, then, "tokens: %v", summarize(model.Modes[0]))
	assert.Equal(t, "then", then.Keyword)
	assert.Equal(t, "", then.Regexp)
}

func TestTextMateModelModeCommands(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token WS: /\s+/
		token mode default {
			"'" -> push(IN_STRING)
			ID
			hidden WS
		}
		token mode IN_STRING {
			"'" -> pop
			token STRING_CONTENT: /[^'#]+/
			"#{" -> push(IN_INTERPOLATION)
		}
		token mode IN_INTERPOLATION {
			"}" -> pop
			ID
			comment WS
			"!" -> mode(default)
		}
	`, true, nil)

	assert.Equal(t, []string{"default", "IN_STRING", "IN_INTERPOLATION"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{{Entry: "Foo", EntryMode: "default"}}, model.Languages)

	def := model.ModesByName["default"]
	inString := model.ModesByName["IN_STRING"]
	inInterpolation := model.ModesByName["IN_INTERPOLATION"]
	quote := def.Tokens[0].Name
	assert.Equal(t, []string{quote + " ->push(IN_STRING)", "ID", "WS:hidden"}, summarize(def))
	assert.Equal(t, []string{quote + " ->pop", inString.Tokens[1].Name + " ->push(IN_INTERPOLATION)", "STRING_CONTENT"}, summarize(inString))
	// The modifier of WS differs per mode.
	assert.Equal(t, []string{"RightBrace ->pop", "Exclamation ->mode(default)", "ID", "WS:comment"}, summarize(inInterpolation))

	assert.Empty(t, def.PopTokens())
	require.Len(t, inString.PopTokens(), 1)
	assert.Equal(t, "'", inString.PopTokens()[0].Keyword)
	require.Len(t, inInterpolation.PopTokens(), 1)
	assert.Equal(t, "}", inInterpolation.PopTokens()[0].Keyword)
}

func TestTextMateModelDropsUnknownTargetMode(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token mode default {
			ID -> push(Missing)
		}
	`, true, nil)

	assert.Equal(t, []string{"ID"}, summarize(model.ModesByName["default"]))
}

func TestTextMateModelKeywordSelector(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: "hello" "world" "+" Name=ID;
		token ID: /[a-z]+/
		token mode default {
			keywords /^[a-z]+$/
			ID
		}
	`, true, nil)

	assert.Equal(t, []string{"hello", "world", "ID"}, summarize(model.ModesByName["default"]))
}

// A multi-language grammar without declared modes gets one mode per language
// with only the tokens reachable from its entry rule.
func TestTextMateModelGeneratedLanguageModes(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" Name=ID;
		entry Farewell: "goodbye" Name=ID;
		token ID: /[a-z]+/
		hidden token WS: /\s+/
	`, true, nil)

	assert.Equal(t, []string{"Greeting", "Farewell"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{
		{Entry: "Greeting", EntryMode: "Greeting"},
		{Entry: "Farewell", EntryMode: "Farewell"},
	}, model.Languages)
	assert.Equal(t, []string{"hello", "ID", "WS:hidden"}, summarize(model.ModesByName["Greeting"]))
	assert.Equal(t, []string{"goodbye", "ID", "WS:hidden"}, summarize(model.ModesByName["Farewell"]))
}

func TestTextMateModelDeclaredLanguageModes(t *testing.T) {
	model := textMateModelFor(t, `
		grammar Test;
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" Name=ID;
		entry Farewell: "goodbye" Name=ID;
		token ID: /[a-z]+/
		hidden token WS: /\s+/
		token mode GreetingMode {
			"hello"
			ID
			hidden WS
		}
		token mode FarewellMode {
			"goodbye"
			ID
			hidden WS
		}
	`, true, []string{"GreetingMode", "FarewellMode"})

	// No implicit default mode is added when modes are declared.
	assert.Equal(t, []string{"GreetingMode", "FarewellMode"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{
		{Entry: "Greeting", EntryMode: "GreetingMode"},
		{Entry: "Farewell", EntryMode: "FarewellMode"},
	}, model.Languages)
	assert.Equal(t, []string{"hello", "ID", "WS:hidden"}, summarize(model.ModesByName["GreetingMode"]))
}
