// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// tokenTypes hands out token types with dense indices, like
// BuildArtifacts.TokenTypes.
type tokenTypes struct {
	all []*tokens.TokenType
}

func newTokenTypes() *tokenTypes {
	return &tokenTypes{all: []*tokens.TokenType{{Index: 0, Name: "EOF", Kind: tokens.KindEOF}}}
}

func (tt *tokenTypes) add(t *tokens.TokenType) *tokens.TokenType {
	t.Index = len(tt.all)
	tt.all = append(tt.all, t)
	return t
}

// keyword adds a keyword without a token declaration.
func (tt *tokenTypes) keyword(value string) *tokens.TokenType {
	return tt.add(&tokens.TokenType{Name: value, Kind: tokens.KindKeyword, Keyword: value})
}

// declaredKeyword adds a keyword named by a token declaration (token A: "a").
func (tt *tokenTypes) declaredKeyword(name, value string) *tokens.TokenType {
	return tt.add(&tokens.TokenType{Name: name, Aliases: []string{value}, Kind: tokens.KindKeyword, Keyword: value})
}

func (tt *tokenTypes) regexp(name, re string, modifier tokens.Modifier) *tokens.TokenType {
	return tt.add(&tokens.TokenType{Name: name, Kind: tokens.KindRegexp, Regexp: re, Modifier: modifier})
}

// modes creates empty token modes with the given names; members are added
// afterwards, since commands may target later modes.
func modes(names ...string) []*tokens.TokenMode {
	result := make([]*tokens.TokenMode, len(names))
	for i, name := range names {
		result[i] = &tokens.TokenMode{Index: i, Name: name}
	}
	return result
}

func member(t *tokens.TokenType, modifier tokens.Modifier) *tokens.TokenModeMember {
	return &tokens.TokenModeMember{TokenType: t, Modifier: modifier}
}

func command(t *tokens.TokenType, kind tokens.CommandKind, target *tokens.TokenMode) *tokens.TokenModeMember {
	return &tokens.TokenModeMember{TokenType: t, Command: &tokens.TokenCommand{Kind: kind, Target: target}}
}

func language(id, entry string, entryMode *tokens.TokenMode) *pluginapi.Language {
	return &pluginapi.Language{LanguageID: id, Entry: entry, EntryMode: entryMode}
}

// summarize renders the tokens of a mode as "Name[:modifier][ ->command(target)]".
func summarize(mode *textMateMode) []string {
	var result []string
	for _, token := range mode.Tokens {
		s := token.Name
		if token.Modifier != tokens.ModifierNone {
			s += ":" + string(token.Modifier)
		}
		if token.Command != "" {
			s += " ->" + string(token.Command)
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
	tt := newTokenTypes()
	hello := tt.keyword("hello")
	semicolon := tt.keyword(";")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ws := tt.regexp("WS", `\s+`, tokens.ModifierHidden)
	comment := tt.regexp("SL_COMMENT", `\/\/[^\n\r]*`, tokens.ModifierComment)
	ms := modes("default")
	ms[0].Members = []*tokens.TokenModeMember{
		member(hello, ""), member(semicolon, ""), member(id, ""),
		member(ws, tokens.ModifierHidden), member(comment, tokens.ModifierComment),
	}

	model := buildTextMateModel(ms, []*pluginapi.Language{language("test", "Foo", ms[0])})

	assert.Equal(t, []string{"default"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{{LanguageID: "test", Entry: "Foo", EntryMode: "default"}}, model.Languages)
	assert.Equal(t, []string{"hello", "Semicolon", "ID", "WS:hidden", "SL_COMMENT:comment"}, summarize(model.Modes[0]))

	got := model.Modes[0].Tokens
	assert.Equal(t, []int{1, 2, 3, 4, 5}, []int{got[0].TokenIndex, got[1].TokenIndex, got[2].TokenIndex, got[3].TokenIndex, got[4].TokenIndex})
	assert.Equal(t, "hello", got[0].Keyword)
	assert.Equal(t, "", got[0].Regexp)
	assert.Equal(t, ";", got[1].Keyword)
	assert.Equal(t, `[a-z]+`, got[2].Regexp)
	assert.Equal(t, "", got[2].Keyword)
	assert.Equal(t, `\/\/[^\n\r]*`, got[4].Regexp)
}

// A token declaration with keyword content keeps its declaration name, even
// if the name equals the keyword value.
func TestTextMateModelKeywordToken(t *testing.T) {
	tt := newTokenTypes()
	then := tt.declaredKeyword("THEN", "then")
	same := tt.declaredKeyword("if", "if")
	// Several declarations of one keyword: the first is the name, the
	// others follow the keyword value in Aliases.
	lparen := tt.declaredKeyword("LPAREN", "(")
	lparen.Aliases = append(lparen.Aliases, "OPEN")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("default")
	ms[0].Members = []*tokens.TokenModeMember{member(then, ""), member(same, ""), member(lparen, ""), member(id, "")}

	model := buildTextMateModel(ms, []*pluginapi.Language{language("test", "Foo", ms[0])})

	assert.Equal(t, []string{"THEN", "if", "LPAREN", "ID"}, summarize(model.Modes[0]))
	assert.Equal(t, "(", model.Modes[0].Tokens[2].Keyword)
	assert.Equal(t, "then", model.Modes[0].Tokens[0].Keyword)
	assert.Equal(t, "", model.Modes[0].Tokens[0].Regexp)
}

func TestTextMateModelModeCommands(t *testing.T) {
	tt := newTokenTypes()
	quote := tt.keyword("'")
	interpolation := tt.keyword("#{")
	rightBrace := tt.keyword("}")
	exclamation := tt.keyword("!")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ws := tt.regexp("WS", `\s+`, tokens.ModifierNone)
	content := tt.regexp("STRING_CONTENT", `[^'#]+`, tokens.ModifierNone)
	ms := modes("default", "IN_STRING", "IN_INTERPOLATION")
	def, inString, inInterpolation := ms[0], ms[1], ms[2]
	def.Members = []*tokens.TokenModeMember{
		command(quote, tokens.CommandPush, inString), member(id, ""), member(ws, tokens.ModifierHidden),
	}
	inString.Members = []*tokens.TokenModeMember{
		command(quote, tokens.CommandPop, nil), command(interpolation, tokens.CommandPush, inInterpolation), member(content, ""),
	}
	inInterpolation.Members = []*tokens.TokenModeMember{
		command(rightBrace, tokens.CommandPop, nil), command(exclamation, tokens.CommandMode, def),
		member(id, ""), member(ws, tokens.ModifierComment),
	}

	model := buildTextMateModel(ms, []*pluginapi.Language{language("test", "Foo", def)})

	assert.Equal(t, []string{"default", "IN_STRING", "IN_INTERPOLATION"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{{LanguageID: "test", Entry: "Foo", EntryMode: "default"}}, model.Languages)

	mDef := model.ModesByName["default"]
	mInString := model.ModesByName["IN_STRING"]
	mInInterpolation := model.ModesByName["IN_INTERPOLATION"]
	assert.Equal(t, []string{"' ->push(IN_STRING)", "ID", "WS:hidden"}, summarize(mDef))
	assert.Equal(t, []string{"' ->pop", "HashLeftBrace ->push(IN_INTERPOLATION)", "STRING_CONTENT"}, summarize(mInString))
	// The modifier of WS differs per mode.
	assert.Equal(t, []string{"RightBrace ->pop", "Exclamation ->mode(default)", "ID", "WS:comment"}, summarize(mInInterpolation))

	assert.Empty(t, mDef.PopTokens())
	require.Len(t, mInString.PopTokens(), 1)
	assert.Equal(t, "'", mInString.PopTokens()[0].Keyword)
	require.Len(t, mInInterpolation.PopTokens(), 1)
	assert.Equal(t, "}", mInInterpolation.PopTokens()[0].Keyword)
}

// A multi-language grammar without declared modes gets one synthetic mode per
// language, named after its entry rule.
func TestTextMateModelSeveralLanguages(t *testing.T) {
	tt := newTokenTypes()
	hello := tt.keyword("hello")
	goodbye := tt.keyword("goodbye")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ws := tt.regexp("WS", `\s+`, tokens.ModifierHidden)
	ms := modes("Greeting", "Farewell")
	ms[0].Synthetic, ms[1].Synthetic = true, true
	ms[0].Members = []*tokens.TokenModeMember{member(hello, ""), member(id, ""), member(ws, tokens.ModifierHidden)}
	ms[1].Members = []*tokens.TokenModeMember{member(goodbye, ""), member(id, ""), member(ws, tokens.ModifierHidden)}

	model := buildTextMateModel(ms, []*pluginapi.Language{
		language("greeting", "Greeting", ms[0]),
		language("farewell", "Farewell", ms[1]),
	})

	assert.Equal(t, []string{"Greeting", "Farewell"}, modeNames(model))
	assert.Equal(t, []textMateLanguage{
		{LanguageID: "greeting", Entry: "Greeting", EntryMode: "Greeting"},
		{LanguageID: "farewell", Entry: "Farewell", EntryMode: "Farewell"},
	}, model.Languages)
	assert.Equal(t, []string{"hello", "ID", "WS:hidden"}, summarize(model.ModesByName["Greeting"]))
	assert.Equal(t, []string{"goodbye", "ID", "WS:hidden"}, summarize(model.ModesByName["Farewell"]))
}
