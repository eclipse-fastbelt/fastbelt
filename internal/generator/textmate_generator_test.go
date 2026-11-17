// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/test"
)

// generateTextMateCommonFor parses src and generates the common TextMate file
// with all entry rules of the grammar as languages.
func generateTextMateCommonFor(t *testing.T, src string, entryModes []string, overrides map[string]string) (TextMateGrammar, TextMateCommonResult) {
	t.Helper()
	grammr := parseGrammar(t, src)
	var entryRules []grammar.ParserRule
	for _, rule := range grammr.Rules() {
		if rule.IsEntry() {
			entryRules = append(entryRules, rule)
		}
	}
	result, err := GenerateTextMateCommon(grammr, entryRules, entryModes, GenerateTokenTypes(grammr), overrides)
	require.NoError(t, err)
	var tm TextMateGrammar
	require.NoError(t, json.Unmarshal([]byte(result.Content), &tm))
	return tm, result
}

func includes(keys ...string) []Pattern {
	result := make([]Pattern, 0, len(keys))
	for _, key := range keys {
		result = append(result, include(key))
	}
	return result
}

func TestTextMateCommonTokens(t *testing.T) {
	tm, result := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: "class" Name=ID ":" ":=" ";" "if" STRING INT;
		token ID: /[a-zA-Z_][a-zA-Z0-9_]*/;
		token INT: /[0-9]+/;
		token STRING: /"(\\.|[^"\\])*"/;
		hidden token WS: /[ \n\r\t]+/;
		hidden token DIRECTIVE: /#[a-z]+/;
		comment token SL_COMMENT: /\/\/[^\r\n]*/;
		comment token ML_COMMENT: /\/\*[\s\S]*?\*\//;
	`, nil, nil)

	require.Empty(t, result.Warnings)
	require.Equal(t, "source.test-common", tm.ScopeName)
	require.Equal(t, []Pattern{}, tm.Patterns)
	require.Equal(t, Pattern{Patterns: includes("mode_default")}, tm.Repository["Foo"])
	// Comments and strings, then keywords, then other tokens. WS is left out.
	require.Equal(t, includes("token_STRING", "token_SL_COMMENT_comment", "token_ML_COMMENT_comment",
		"keywords_default", "token_ID", "token_INT", "token_DIRECTIVE_hidden"), tm.Repository["mode_default"].Patterns)
	require.NotContains(t, tm.Repository, "token_WS_hidden")

	require.Equal(t, Pattern{
		Match: `(\b(?:class|if)\b)|(:=)|(:)|(;)`,
		Captures: Captures{
			"1": {Name: "keyword.control.test"},
			"2": {Name: "keyword.operator.test"},
			"3": {Name: "punctuation.separator.test"},
			"4": {Name: "punctuation.terminator.test"},
		},
	}, tm.Repository["keywords_default"])
	require.Equal(t, Pattern{Match: `[A-Z_a-z][0-9A-Z_a-z]*`}, tm.Repository["token_ID"])
	require.Equal(t, Pattern{Match: `[0-9]+`, Name: "constant.numeric.test"}, tm.Repository["token_INT"])
	require.Equal(t, Pattern{Match: `#[a-z]+`}, tm.Repository["token_DIRECTIVE_hidden"])
	require.Equal(t, Pattern{
		Name: "string.quoted.double.test", Begin: `"`, End: `"`,
		Patterns: []Pattern{{Match: `\\.`, Name: "constant.character.escape.test"}},
	}, tm.Repository["token_STRING"])
	require.Equal(t, Pattern{Name: "comment.line.test", Match: `//[^\n\r]*`}, tm.Repository["token_SL_COMMENT_comment"])
	require.Equal(t, Pattern{Name: "comment.block.test", Begin: `/\*`, End: `\*/`}, tm.Repository["token_ML_COMMENT_comment"])
}

func TestTextMateCommonStringAlternatives(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=STRING;
		token STRING: /"[^"]*"|'[^']*'/;
		token TEXT_STRING: /[a-z]+/;
	`, nil, nil)

	require.Equal(t, Pattern{Patterns: []Pattern{
		{Name: "string.quoted.double.test", Begin: `"`, End: `"`},
		{Name: "string.quoted.single.test", Begin: `'`, End: `'`},
	}}, tm.Repository["token_STRING"])
	// No begin delimiter: a single match.
	require.Equal(t, Pattern{Name: "string.quoted.other.test", Match: `[a-z]+`}, tm.Repository["token_TEXT_STRING"])
}

func TestTextMateCommonCommentWithLineAndBlockAlternatives(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/;
		comment token COMMENT: /--[^\n]*|\{-[\s\S]*?-\}/;
	`, nil, nil)

	require.Equal(t, Pattern{Patterns: []Pattern{
		{Name: "comment.line.test", Begin: `--`, End: `$`},
		{Name: "comment.block.test", Begin: `\{-`, End: `-\}`},
	}}, tm.Repository["token_COMMENT_comment"])
}

func TestTextMateCommonOverrides(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: "if" Name=ID INT ";";
		token ID: /[a-z]+/;
		token INT: /[0-9]+/;
		hidden token WS: /\s+/;
	`, nil, map[string]string{"ID": "variable.other", "INT": "", "Semicolon": "punctuation.other", "WS": "invalid"})

	require.Equal(t, Pattern{Match: `[a-z]+`, Name: "variable.other.test"}, tm.Repository["token_ID"])
	require.Equal(t, Pattern{Match: `[0-9]+`}, tm.Repository["token_INT"])
	require.Equal(t, Pattern{
		Match:    `(\bif\b)|(;)`,
		Captures: Captures{"1": {Name: "keyword.control.test"}, "2": {Name: "punctuation.other.test"}},
	}, tm.Repository["keywords_default"])
	require.NotContains(t, tm.Repository, "token_WS_hidden")
}

func TestTextMateCommonPushAndPop(t *testing.T) {
	tm, result := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token mode default {
			"` + "`" + `" -> push(IN_STRING)
			"+"
			ID
		}
		token mode IN_STRING {
			"` + "`" + `" -> pop
			token STRING_CONTENT: /[^` + "`" + `#]+/
			"#{" -> push(IN_INTERPOLATION)
		}
		token mode IN_INTERPOLATION {
			"}" -> pop
			"` + "`" + `" -> push(IN_STRING)
			"+"
			ID
		}
	`, nil, nil)

	require.Empty(t, result.Warnings)
	require.Equal(t, includes("token_Backtick_push_IN_STRING", "keywords_default", "token_ID"),
		tm.Repository["mode_default"].Patterns)
	// The pop token is the end of the push rule, not a pattern of the mode.
	require.Equal(t, includes("token_HashLeftBrace_push_IN_INTERPOLATION", "token_STRING_CONTENT"),
		tm.Repository["mode_IN_STRING"].Patterns)
	require.Equal(t, includes("token_Backtick_push_IN_STRING", "keywords_IN_INTERPOLATION", "token_ID"),
		tm.Repository["mode_IN_INTERPOLATION"].Patterns)
	require.Equal(t, Pattern{
		Begin:         "`",
		BeginCaptures: Captures{"0": {Name: "keyword.operator.test"}},
		End:           "`",
		EndCaptures:   Captures{"0": {Name: "keyword.operator.test"}},
		Patterns:      includes("mode_IN_STRING"),
	}, tm.Repository["token_Backtick_push_IN_STRING"])
	require.Equal(t, Pattern{
		Begin:         `#\{`,
		BeginCaptures: Captures{"0": {Name: "keyword.operator.test"}},
		End:           `\}`,
		EndCaptures:   Captures{"0": {Name: "punctuation.section.test"}},
		Patterns:      includes("mode_IN_INTERPOLATION"),
	}, tm.Repository["token_HashLeftBrace_push_IN_INTERPOLATION"])
}

func TestTextMateCommonPushWithoutPop(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token mode default {
			"begin" -> push(REST)
			ID
		}
		token mode REST {
			ID
		}
	`, nil, nil)

	require.Equal(t, Pattern{
		Begin:         `\bbegin\b`,
		BeginCaptures: Captures{"0": {Name: "keyword.control.test"}},
		End:           `(?!)`,
		Patterns:      includes("mode_REST"),
	}, tm.Repository["token_begin_push_REST"])
}

func TestTextMateCommonMultiplePopTokens(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token mode default {
			"<" -> push(INNER)
			ID
		}
		token mode INNER {
			token A_STRING: /(x)y/ -> pop
			token B_INT: /[0-9]+z/ -> pop
			ID
		}
	`, nil, nil)

	// The capture inside A_STRING shifts the group of B_INT.
	require.Equal(t, Pattern{
		Begin:         `<`,
		BeginCaptures: Captures{"0": {Name: "keyword.operator.test"}},
		End:           `((x)y)|([0-9]+z)`,
		EndCaptures:   Captures{"1": {Name: "string.quoted.other.test"}, "3": {Name: "constant.numeric.test"}},
		Patterns:      includes("mode_INNER"),
	}, tm.Repository["token_LessThan_push_INNER"])
}

func TestTextMateCommonModeCommand(t *testing.T) {
	tm, result := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token mode default {
			"(" -> push(A)
			ID
		}
		token mode A {
			")" -> pop
			"!" -> mode(B)
			"?" -> mode(C)
			ID
		}
		token mode B {
			")" -> pop
			ID
		}
		token mode C {
			"]" -> pop
			ID
		}
	`, nil, nil)

	require.Equal(t, includes("token_Exclamation_mode_B_in_A", "token_Question_mode_C_in_A", "token_ID"),
		tm.Repository["mode_A"].Patterns)
	require.Equal(t, Pattern{
		Begin:         `!`,
		BeginCaptures: Captures{"0": {Name: "keyword.operator.test"}},
		End:           `(?=\))`,
		Patterns:      includes("mode_B"),
	}, tm.Repository["token_Exclamation_mode_B_in_A"])
	require.Equal(t, `(?=\))`, tm.Repository["token_Question_mode_C_in_A"].End)
	// Only C has other pop tokens than A.
	require.Len(t, result.Warnings, 1)
	require.Contains(t, result.Warnings[0], "from token mode A to C")
}

func TestTextMateCommonModeCommandInRootMode(t *testing.T) {
	tm, result := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token mode default {
			"!" -> mode(OTHER)
			ID
		}
		token mode OTHER {
			ID
		}
	`, nil, nil)

	require.Empty(t, result.Warnings)
	require.Equal(t, `(?!)`, tm.Repository["token_Exclamation_mode_OTHER_in_default"].End)
}

func TestTextMateCommonModifierPerMode(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Test;
		interface Foo { Name string }
		entry Foo: Name=ID;
		token ID: /[a-z]+/
		token NOTE: /#[a-z]+/
		token mode default {
			"(" -> push(A)
			ID
			hidden NOTE
		}
		token mode A {
			")" -> pop
			comment NOTE
		}
	`, nil, nil)

	require.Equal(t, Pattern{Match: `#[a-z]+`}, tm.Repository["token_NOTE_hidden"])
	require.Equal(t, Pattern{Match: `#[a-z]+`, Name: "comment.line.test"}, tm.Repository["token_NOTE_comment"])
}

func TestTextMateCommonLanguages(t *testing.T) {
	tm, _ := generateTextMateCommonFor(t, `
		grammar Multi;
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" Name=ID;
		entry Farewell: "goodbye" Name=ID;
		token ID: /[a-z]+/;
	`, nil, nil)

	require.Equal(t, "source.multi-common", tm.ScopeName)
	require.Equal(t, Pattern{Patterns: includes("mode_Greeting")}, tm.Repository["Greeting"])
	require.Equal(t, Pattern{Patterns: includes("mode_Farewell")}, tm.Repository["Farewell"])
	require.Equal(t, Pattern{Match: `\bhello\b`, Name: "keyword.control.multi"}, tm.Repository["keywords_Greeting"])
	require.Equal(t, Pattern{Match: `\bgoodbye\b`, Name: "keyword.control.multi"}, tm.Repository["keywords_Farewell"])
}

func TestTextMateCommonDeterministic(t *testing.T) {
	src := `
		grammar Test;
		interface Foo { Name string }
		entry Foo: "a" "b" "&&" "<" Name=ID;
		token ID: /[a-z]+/;
		token STRING: /"[^"]*"/;
	`
	_, first := generateTextMateCommonFor(t, src, nil, nil)
	for range 5 {
		_, next := generateTextMateCommonFor(t, src, nil, nil)
		require.Equal(t, first.Content, next.Content)
	}
	// HTML characters are not escaped.
	require.Contains(t, first.Content, `(&&|<)`)
}

func parseGrammar(t *testing.T, src string) grammar.Grammar {
	t.Helper()
	f := test.New(t, grammar.CreateServices())
	doc := f.Parse(src)
	grammr, ok := doc.Document.Root.(grammar.Grammar)
	require.True(t, ok)
	return grammr
}

func TestTextMateLanguage(t *testing.T) {
	grammr := parseGrammar(t, `
		grammar MultilangModel;
		interface Greeting { Name string }
		entry Greeting: "hello" Name=ID;
		token ID: /[a-z]+/;
	`)
	content, err := GenerateTextMateLanguage(grammr, "Greeting", Selector{LanguageID: "hello", Patterns: []string{"*.hello", "**/*.hi"}})
	require.NoError(t, err)
	require.Equal(t, `{
  "name": "hello",
  "scopeName": "source.hello",
  "fileTypes": [
    "hello",
    "hi"
  ],
  "patterns": [
    {
      "include": "source.multilangmodel-common#Greeting"
    }
  ]
}
`, content)
	require.Equal(t, "multilangmodel-common.tmLanguage.json", TextMateCommonFileName(grammr))
	require.Equal(t, "hello.tmLanguage.json", TextMateLanguageFileName("hello"))
}

func TestTextMateLanguageWithoutPatterns(t *testing.T) {
	grammr := parseGrammar(t, `grammar Test; token ID: /[a-z]+/;`)
	content, err := GenerateTextMateLanguage(grammr, "Foo", Selector{LanguageID: "test"})
	require.NoError(t, err)
	require.NotContains(t, content, "fileTypes")
}

func TestTextMateLanguageErrors(t *testing.T) {
	grammr := parseGrammar(t, `grammar Test; token ID: /[a-z]+/;`)
	_, err := GenerateTextMateLanguage(grammr, "", Selector{LanguageID: "test"})
	require.Error(t, err)
	_, err = GenerateTextMateLanguage(grammr, "Foo", Selector{})
	require.Error(t, err)
}

func TestTextMateFileTypes(t *testing.T) {
	require.Nil(t, textMateFileTypes(nil))
	require.Equal(t, []string{"calc", "c", "h", "tar.gz"}, textMateFileTypes([]string{
		"*.calc", "**/*.calc", "*.{c,h}", "*.tar.gz",
		// Not simple extensions:
		"Makefile", "src/*.x", "*.{a,}", "*.{a,b", "*.[ch]", "*.x*", "foo.*",
	}))
}
