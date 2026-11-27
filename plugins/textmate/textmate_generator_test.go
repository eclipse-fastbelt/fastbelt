// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

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

// artifactsFor returns build artifacts with the given grammar name, modes
// and languages, and the handler that collects the messages of its logger.
// Without languages, there is one language "test" with entry Foo in the first
// mode.
func artifactsFor(grammarName string, ms []*tokens.TokenMode, languages ...*pluginapi.Language) (pluginapi.BuildArtifacts, *collectingHandler) {
	if len(languages) == 0 {
		languages = []*pluginapi.Language{language("test", "Foo", ms[0])}
	}
	handler := &collectingHandler{Handler: slog.DiscardHandler}
	return pluginapi.BuildArtifacts{
		Grammar:    &ast.Grammar{Name: grammarName},
		TokenModes: ms,
		Languages:  languages,
		Logger:     slog.New(handler),
	}, handler
}

// generateCommonFor generates the common TextMate file of a grammar named
// Test (see artifactsFor) and returns it decoded, its content and the logged
// warnings.
func generateCommonFor(t *testing.T, ms []*tokens.TokenMode, overrides map[string]string, languages ...*pluginapi.Language) (tmGrammar, string, []string) {
	t.Helper()
	artifacts, handler := artifactsFor("Test", ms, languages...)
	return generateCommonFrom(t, artifacts, handler, overrides)
}

func generateCommonFrom(t *testing.T, artifacts pluginapi.BuildArtifacts, handler *collectingHandler, overrides map[string]string) (tmGrammar, string, []string) {
	t.Helper()
	content, err := generateCommon(artifacts, overrides)
	require.NoError(t, err)
	var tm tmGrammar
	require.NoError(t, json.Unmarshal([]byte(content), &tm))
	return tm, content, handler.messages
}

// members returns the tokens as members without modifier or command.
func members(tts ...*tokens.TokenType) []*tokens.TokenModeMember {
	result := make([]*tokens.TokenModeMember, 0, len(tts))
	for _, tt := range tts {
		result = append(result, member(tt, tokens.ModifierNone))
	}
	return result
}

func includes(keys ...string) []tmPattern {
	result := make([]tmPattern, 0, len(keys))
	for _, key := range keys {
		result = append(result, include(key))
	}
	return result
}

func TestTextMateCommonTokens(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = append(members(
		tt.keyword("class"), tt.keyword(":"), tt.keyword(":="), tt.keyword(";"), tt.keyword("if"),
		tt.regexp("ID", `[a-zA-Z_][a-zA-Z0-9_]*`, tokens.ModifierNone),
		tt.regexp("INT", `[0-9]+`, tokens.ModifierNone),
		tt.regexp("STRING", `"(\\.|[^"\\])*"`, tokens.ModifierNone),
	),
		member(tt.regexp("WS", `[ \n\r\t]+`, tokens.ModifierHidden), tokens.ModifierHidden),
		member(tt.regexp("DIRECTIVE", `#[a-z]+`, tokens.ModifierHidden), tokens.ModifierHidden),
		member(tt.regexp("SL_COMMENT", `\/\/[^\r\n]*`, tokens.ModifierComment), tokens.ModifierComment),
		member(tt.regexp("ML_COMMENT", `\/\*[\s\S]*?\*\/`, tokens.ModifierComment), tokens.ModifierComment),
	)
	tm, _, warnings := generateCommonFor(t, ms, nil)

	require.Empty(t, warnings)
	require.Equal(t, "source.test-common", tm.ScopeName)
	require.Equal(t, []tmPattern{}, tm.Patterns)
	require.Equal(t, tmPattern{Patterns: includes("mode_default")}, tm.Repository["Foo"])
	// Strings, hidden tokens and comments, then keywords, then other tokens. WS is left out.
	require.Equal(t, includes("token_STRING", "token_DIRECTIVE_hidden", "token_SL_COMMENT_comment", "token_ML_COMMENT_comment",
		"keywords_default", "token_ID", "token_INT"), tm.Repository["mode_default"].Patterns)
	require.NotContains(t, tm.Repository, "token_WS_hidden")

	require.Equal(t, tmPattern{
		Match: `(\b(?:class|if)\b)|(:=)|(:)|(;)`,
		Captures: tmCaptures{
			"1": {Name: "keyword.control.test"},
			"2": {Name: "keyword.operator.test"},
			"3": {Name: "punctuation.separator.test"},
			"4": {Name: "punctuation.terminator.test"},
		},
	}, tm.Repository["keywords_default"])
	require.Equal(t, tmPattern{Match: `[A-Z_a-z][0-9A-Z_a-z]*`}, tm.Repository["token_ID"])
	require.Equal(t, tmPattern{Match: `[0-9]+`, Name: "constant.numeric.test"}, tm.Repository["token_INT"])
	require.Equal(t, tmPattern{Match: `#[a-z]+`}, tm.Repository["token_DIRECTIVE_hidden"])
	require.Equal(t, tmPattern{
		Name: "string.quoted.double.test", Begin: `"`, End: `"`,
		Patterns: []tmPattern{{Match: `\\.`, Name: "constant.character.escape.test"}},
	}, tm.Repository["token_STRING"])
	require.Equal(t, tmPattern{Name: "comment.line.test", Match: `//[^\n\r]*`}, tm.Repository["token_SL_COMMENT_comment"])
	require.Equal(t, tmPattern{Name: "comment.block.test", Begin: `/\*`, End: `\*/`}, tm.Repository["token_ML_COMMENT_comment"])
}

func TestTextMateCommonStringAlternatives(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(
		tt.regexp("STRING", `"[^"]*"|'[^']*'`, tokens.ModifierNone),
		tt.regexp("TEXT_STRING", `[a-z]+`, tokens.ModifierNone),
	)
	tm, _, _ := generateCommonFor(t, ms, nil)

	require.Equal(t, tmPattern{Patterns: []tmPattern{
		{Name: "string.quoted.double.test", Begin: `"`, End: `"`},
		{Name: "string.quoted.single.test", Begin: `'`, End: `'`},
	}}, tm.Repository["token_STRING"])
	// No begin delimiter: a single match.
	require.Equal(t, tmPattern{Name: "string.quoted.other.test", Match: `[a-z]+`}, tm.Repository["token_TEXT_STRING"])
}

func TestTextMateCommonCommentWithLineAndBlockAlternatives(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = []*tokens.TokenModeMember{
		member(tt.regexp("ID", `[a-z]+`, tokens.ModifierNone), tokens.ModifierNone),
		member(tt.regexp("COMMENT", `--[^\n]*|\{-[\s\S]*?-\}`, tokens.ModifierComment), tokens.ModifierComment),
	}
	tm, _, _ := generateCommonFor(t, ms, nil)

	require.Equal(t, tmPattern{Patterns: []tmPattern{
		{Name: "comment.line.test", Begin: `--`, End: `$`},
		{Name: "comment.block.test", Begin: `\{-`, End: `-\}`},
	}}, tm.Repository["token_COMMENT_comment"])
}

func TestTextMateCommonOverrides(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = append(members(
		tt.keyword("if"), tt.keyword(";"),
		tt.regexp("ID", `[a-z]+`, tokens.ModifierNone),
		tt.regexp("INT", `[0-9]+`, tokens.ModifierNone),
	),
		member(tt.regexp("WS", `\s+`, tokens.ModifierHidden), tokens.ModifierHidden),
		member(tt.regexp("NOTE", `%[a-z]*`, tokens.ModifierHidden), tokens.ModifierHidden),
	)
	tm, _, warnings := generateCommonFor(t, ms,
		map[string]string{"ID": "variable.other", "INT": "", "Semicolon": "punctuation.other", "NOTE": "comment.line"})

	require.Empty(t, warnings)
	require.Equal(t, tmPattern{Match: `[a-z]+`, Name: "variable.other.test"}, tm.Repository["token_ID"])
	require.Equal(t, tmPattern{Match: `[0-9]+`}, tm.Repository["token_INT"])
	require.Equal(t, tmPattern{
		Match:    `(\bif\b)|(;)`,
		Captures: tmCaptures{"1": {Name: "keyword.control.test"}, "2": {Name: "punctuation.other.test"}},
	}, tm.Repository["keywords_default"])
	// Hidden tokens get a scope only through an override.
	require.Equal(t, tmPattern{Match: `%[a-z]*`, Name: "comment.line.test"}, tm.Repository["token_NOTE_hidden"])
	require.NotContains(t, tm.Repository, "token_WS_hidden")
}

// A hidden line comment must win over a "/" keyword, like in arithmetics.
func TestTextMateCommonHiddenCommentBeforeKeywords(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = append(members(tt.keyword("/"), tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)),
		member(tt.regexp("WS", `\s+`, tokens.ModifierHidden), tokens.ModifierHidden),
		member(tt.regexp("SL_COMMENT", `\/\/[^\n\r]*`, tokens.ModifierHidden), tokens.ModifierHidden),
		member(tt.regexp("ML_COMMENT", `\/\*[\s\S]*?\*\/`, tokens.ModifierHidden), tokens.ModifierHidden),
	)
	tm, _, _ := generateCommonFor(t, ms, map[string]string{"SL_COMMENT": "comment.line"})

	require.Equal(t, includes("token_SL_COMMENT_hidden", "token_ML_COMMENT_hidden", "keywords_default", "token_ID"),
		tm.Repository["mode_default"].Patterns)
	require.Equal(t, tmPattern{Match: `//[^\n\r]*`, Name: "comment.line.test"}, tm.Repository["token_SL_COMMENT_hidden"])
	// Without a scope, a hidden block still spans lines, so keywords inside it are not highlighted.
	require.Equal(t, tmPattern{Begin: `/\*`, End: `\*/`}, tm.Repository["token_ML_COMMENT_hidden"])
}

func TestTextMateCommonPushAndPop(t *testing.T) {
	tt := newTokenTypes()
	backtick := tt.keyword("`")
	plus := tt.keyword("+")
	interpolation := tt.keyword("#{")
	rightBrace := tt.keyword("}")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	content := tt.regexp("STRING_CONTENT", "[^`#]+", tokens.ModifierNone)
	ms := modes("default", "IN_STRING", "IN_INTERPOLATION")
	def, inString, inInterpolation := ms[0], ms[1], ms[2]
	def.Members = append([]*tokens.TokenModeMember{command(backtick, tokens.CommandPush, inString)}, members(plus, id)...)
	inString.Members = []*tokens.TokenModeMember{
		command(backtick, tokens.CommandPop, nil),
		member(content, tokens.ModifierNone),
		command(interpolation, tokens.CommandPush, inInterpolation),
	}
	inInterpolation.Members = append([]*tokens.TokenModeMember{
		command(rightBrace, tokens.CommandPop, nil),
		command(backtick, tokens.CommandPush, inString),
	}, members(plus, id)...)
	tm, _, warnings := generateCommonFor(t, ms, nil)

	require.Empty(t, warnings)
	require.Equal(t, includes("token_Backtick_push_IN_STRING", "keywords_default", "token_ID"),
		tm.Repository["mode_default"].Patterns)
	// The pop token is the end of the push rule, not a pattern of the mode.
	require.Equal(t, includes("token_HashLeftBrace_push_IN_INTERPOLATION", "token_STRING_CONTENT"),
		tm.Repository["mode_IN_STRING"].Patterns)
	require.Equal(t, includes("token_Backtick_push_IN_STRING", "keywords_IN_INTERPOLATION", "token_ID"),
		tm.Repository["mode_IN_INTERPOLATION"].Patterns)
	require.Equal(t, tmPattern{
		Begin:         "`",
		BeginCaptures: tmCaptures{"0": {Name: "keyword.operator.test"}},
		End:           "`",
		EndCaptures:   tmCaptures{"0": {Name: "keyword.operator.test"}},
		Patterns:      includes("mode_IN_STRING"),
	}, tm.Repository["token_Backtick_push_IN_STRING"])
	require.Equal(t, tmPattern{
		Begin:         `#\{`,
		BeginCaptures: tmCaptures{"0": {Name: "keyword.operator.test"}},
		End:           `\}`,
		EndCaptures:   tmCaptures{"0": {Name: "punctuation.section.test"}},
		Patterns:      includes("mode_IN_INTERPOLATION"),
	}, tm.Repository["token_HashLeftBrace_push_IN_INTERPOLATION"])
}

func TestTextMateCommonPushWithoutPop(t *testing.T) {
	tt := newTokenTypes()
	begin := tt.keyword("begin")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("default", "REST")
	ms[0].Members = append([]*tokens.TokenModeMember{command(begin, tokens.CommandPush, ms[1])}, members(id)...)
	ms[1].Members = members(id)
	tm, _, _ := generateCommonFor(t, ms, nil)

	require.Equal(t, tmPattern{
		Begin:         `\bbegin\b`,
		BeginCaptures: tmCaptures{"0": {Name: "keyword.control.test"}},
		End:           `(?!)`,
		Patterns:      includes("mode_REST"),
	}, tm.Repository["token_begin_push_REST"])
}

func TestTextMateCommonMultiplePopTokens(t *testing.T) {
	tt := newTokenTypes()
	lessThan := tt.keyword("<")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	aString := tt.regexp("A_STRING", `(x)y`, tokens.ModifierNone)
	bInt := tt.regexp("B_INT", `[0-9]+z`, tokens.ModifierNone)
	ms := modes("default", "INNER")
	ms[0].Members = append([]*tokens.TokenModeMember{command(lessThan, tokens.CommandPush, ms[1])}, members(id)...)
	ms[1].Members = append([]*tokens.TokenModeMember{
		command(aString, tokens.CommandPop, nil),
		command(bInt, tokens.CommandPop, nil),
	}, members(id)...)
	tm, _, _ := generateCommonFor(t, ms, nil)

	// The capture inside A_STRING shifts the group of B_INT.
	require.Equal(t, tmPattern{
		Begin:         `<`,
		BeginCaptures: tmCaptures{"0": {Name: "keyword.operator.test"}},
		End:           `((x)y)|([0-9]+z)`,
		EndCaptures:   tmCaptures{"1": {Name: "string.quoted.other.test"}, "3": {Name: "constant.numeric.test"}},
		Patterns:      includes("mode_INNER"),
	}, tm.Repository["token_LessThan_push_INNER"])
}

func TestTextMateCommonModeCommand(t *testing.T) {
	tt := newTokenTypes()
	leftParen := tt.keyword("(")
	rightParen := tt.keyword(")")
	exclamation := tt.keyword("!")
	question := tt.keyword("?")
	rightBracket := tt.keyword("]")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("default", "A", "B", "C")
	def, a, b, c := ms[0], ms[1], ms[2], ms[3]
	def.Members = append([]*tokens.TokenModeMember{command(leftParen, tokens.CommandPush, a)}, members(id)...)
	a.Members = append([]*tokens.TokenModeMember{
		command(rightParen, tokens.CommandPop, nil),
		command(exclamation, tokens.CommandMode, b),
		command(question, tokens.CommandMode, c),
	}, members(id)...)
	b.Members = append([]*tokens.TokenModeMember{command(rightParen, tokens.CommandPop, nil)}, members(id)...)
	c.Members = append([]*tokens.TokenModeMember{command(rightBracket, tokens.CommandPop, nil)}, members(id)...)
	tm, _, warnings := generateCommonFor(t, ms, nil)

	require.Equal(t, includes("token_Exclamation_mode_B_in_A", "token_Question_mode_C_in_A", "token_ID"),
		tm.Repository["mode_A"].Patterns)
	require.Equal(t, tmPattern{
		Begin:         `!`,
		BeginCaptures: tmCaptures{"0": {Name: "keyword.operator.test"}},
		End:           `(?=\))`,
		Patterns:      includes("mode_B"),
	}, tm.Repository["token_Exclamation_mode_B_in_A"])
	require.Equal(t, `(?=\))`, tm.Repository["token_Question_mode_C_in_A"].End)
	// Only C has other pop tokens than A.
	require.Equal(t, []string{
		"TextMate: Question switches from token mode A to C, which have different pop tokens; " +
			"highlighting of C ends at the pop tokens of A",
	}, warnings)
}

func TestTextMateCommonModeCommandInRootMode(t *testing.T) {
	tt := newTokenTypes()
	exclamation := tt.keyword("!")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("default", "OTHER")
	ms[0].Members = append([]*tokens.TokenModeMember{command(exclamation, tokens.CommandMode, ms[1])}, members(id)...)
	ms[1].Members = members(id)
	tm, _, warnings := generateCommonFor(t, ms, nil)

	require.Empty(t, warnings)
	require.Equal(t, `(?!)`, tm.Repository["token_Exclamation_mode_OTHER_in_default"].End)
}

func TestTextMateCommonModifierPerMode(t *testing.T) {
	tt := newTokenTypes()
	leftParen := tt.keyword("(")
	rightParen := tt.keyword(")")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	note := tt.regexp("NOTE", `#[a-z]+`, tokens.ModifierNone)
	ms := modes("default", "A")
	ms[0].Members = []*tokens.TokenModeMember{
		command(leftParen, tokens.CommandPush, ms[1]), member(id, tokens.ModifierNone), member(note, tokens.ModifierHidden),
	}
	ms[1].Members = []*tokens.TokenModeMember{command(rightParen, tokens.CommandPop, nil), member(note, tokens.ModifierComment)}
	tm, _, _ := generateCommonFor(t, ms, nil)

	require.Equal(t, tmPattern{Match: `#[a-z]+`}, tm.Repository["token_NOTE_hidden"])
	require.Equal(t, tmPattern{Match: `#[a-z]+`, Name: "comment.line.test"}, tm.Repository["token_NOTE_comment"])
}

func TestTextMateCommonLanguages(t *testing.T) {
	tt := newTokenTypes()
	hello := tt.keyword("hello")
	goodbye := tt.keyword("goodbye")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("Greeting", "Farewell")
	ms[0].Members = members(hello, id)
	ms[1].Members = members(goodbye, id)
	artifacts, handler := artifactsFor("Multi", ms,
		language("greeting", "Greeting", ms[0]), language("farewell", "Farewell", ms[1]))
	tm, _, _ := generateCommonFrom(t, artifacts, handler, nil)

	require.Equal(t, "source.multi-common", tm.ScopeName)
	require.Equal(t, tmPattern{Patterns: includes("mode_Greeting")}, tm.Repository["Greeting"])
	require.Equal(t, tmPattern{Patterns: includes("mode_Farewell")}, tm.Repository["Farewell"])
	require.Equal(t, tmPattern{Match: `\bhello\b`, Name: "keyword.control.multi"}, tm.Repository["keywords_Greeting"])
	require.Equal(t, tmPattern{Match: `\bgoodbye\b`, Name: "keyword.control.multi"}, tm.Repository["keywords_Farewell"])
}

func TestTextMateCommonDeterministic(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(
		tt.keyword("a"), tt.keyword("b"), tt.keyword("&&"), tt.keyword("<"),
		tt.regexp("ID", `[a-z]+`, tokens.ModifierNone),
		tt.regexp("STRING", `"[^"]*"`, tokens.ModifierNone),
	)
	_, first, _ := generateCommonFor(t, ms, nil)
	for range 5 {
		_, next, _ := generateCommonFor(t, ms, nil)
		require.Equal(t, first, next)
	}
	// HTML characters are not escaped.
	require.Contains(t, first, `(&&|<)`)
}

func TestTextMateCommonInvalidRegexp(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(tt.regexp("BAD", `[a-`, tokens.ModifierNone))
	artifacts, _ := artifactsFor("Test", ms)

	_, err := generateCommon(artifacts, nil)
	require.ErrorContains(t, err, "token BAD")
}

func TestTextMateCommonWarnsAboutUnknownOverrides(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(tt.regexp("ID", `[a-z]+`, tokens.ModifierNone))
	_, _, warnings := generateCommonFor(t, ms, map[string]string{"ID": "variable", "Nope": "x", "Also": "y"})

	require.Equal(t, []string{
		"TextMate: scope override for unknown token Also",
		"TextMate: scope override for unknown token Nope",
	}, warnings)
}

func TestTextMateLanguage(t *testing.T) {
	ms := modes("default")
	greeting := &pluginapi.Language{LanguageID: "hello", Entry: "Greeting", Patterns: []string{"*.hello", "**/*.hi"}, EntryMode: ms[0]}
	artifacts, _ := artifactsFor("MultilangModel", ms, greeting)

	content, err := generateLanguage(artifacts, greeting)
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
}

func TestTextMateLanguageWithoutPatterns(t *testing.T) {
	ms := modes("default")
	artifacts, _ := artifactsFor("Test", ms)

	content, err := generateLanguage(artifacts, artifacts.Languages[0])
	require.NoError(t, err)
	require.NotContains(t, content, "fileTypes")
}

func TestTextMateFileTypes(t *testing.T) {
	require.Nil(t, textMateFileTypes(nil))
	require.Equal(t, []string{"calc", "c", "h", "tar.gz"}, textMateFileTypes([]string{
		"*.calc", "**/*.calc", "*.{c,h}", "*.tar.gz",
		// Not simple extensions:
		"Makefile", "src/*.x", "*.{a,}", "*.{a,b", "*.[ch]", "*.x*", "foo.*",
	}))
}
