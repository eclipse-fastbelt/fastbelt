// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// textMateModel describes the token modes of a grammar as the TextMate
// generator needs them. It mirrors the modes of the generated lexer
// ([pluginapi.BuildArtifacts.TokenModes]), so highlighting follows the same
// mode switches as lexing.
type textMateModel struct {
	// Modes in the order of the generated lexer.
	Modes []*textMateMode
	// ModesByName maps a mode name to its mode.
	ModesByName map[string]*textMateMode
	// Languages are the languages of the build, in configured order.
	Languages []textMateLanguage
}

// textMateLanguage is one language of the build.
type textMateLanguage struct {
	// LanguageID is the language ID of the language.
	LanguageID string
	// Entry is the name of the entry rule.
	Entry string
	// EntryMode is the name of the mode the language starts in.
	EntryMode string
}

// textMateMode is one token mode.
type textMateMode struct {
	// Name of the mode: the declared name, "default", or for synthetic
	// per-language modes the name of the entry rule.
	Name string
	// Tokens that are active in this mode, in lexer priority order.
	Tokens []textMateToken
}

// PopTokens returns the tokens of m that pop m from the mode stack. TextMate
// uses them as the end pattern of the rules that push m.
func (m *textMateMode) PopTokens() []textMateToken {
	var result []textMateToken
	for _, token := range m.Tokens {
		if token.Command == tokens.CommandPop {
			result = append(result, token)
		}
	}
	return result
}

// textMateToken is one token type as it is used in one mode.
type textMateToken struct {
	// TokenIndex is the token index of the generated lexer.
	TokenIndex int
	// Name is the token declaration name. For keywords without a token
	// declaration it is the generated keyword name (see keywordName).
	Name string
	// Keyword is the unquoted keyword value for keywords and token
	// declarations with keyword content, "" for regexp tokens.
	Keyword string
	// Regexp is the regex without slashes for regexp tokens, "" for keywords.
	Regexp string
	// Modifier is the modifier in this mode.
	Modifier tokens.Modifier
	// Command is the mode command in this mode, "" if there is none.
	Command tokens.CommandKind
	// Target is the target mode name of a push or mode command.
	Target string
}

// buildTextMateModel collects the token modes and languages of a build, see
// [pluginapi.BuildArtifacts.TokenModes] and
// [pluginapi.BuildArtifacts.Languages].
func buildTextMateModel(modes []*tokens.TokenMode, languages []*pluginapi.Language) textMateModel {
	model := textMateModel{ModesByName: map[string]*textMateMode{}}
	for _, m := range modes {
		mode := &textMateMode{Name: m.Name}
		for _, member := range m.Members {
			mode.Tokens = append(mode.Tokens, newTextMateToken(member))
		}
		model.Modes = append(model.Modes, mode)
		model.ModesByName[mode.Name] = mode
	}
	for _, language := range languages {
		model.Languages = append(model.Languages, textMateLanguage{
			LanguageID: language.LanguageID,
			Entry:      language.Entry,
			EntryMode:  language.EntryMode.Name,
		})
	}
	return model
}

// newTextMateToken converts a member of a token mode. Keywords without a
// token declaration are named by keywordName.
func newTextMateToken(member *tokens.TokenModeMember) textMateToken {
	tokenType := member.TokenType
	token := textMateToken{
		TokenIndex: tokenType.Index,
		Name:       tokenType.Name,
		Modifier:   member.Modifier,
	}
	switch tokenType.Kind {
	case tokens.KindKeyword:
		token.Keyword = tokenType.Keyword
		// A keyword named by a token declaration lists its value in Aliases
		// and keeps the declaration name.
		if len(tokenType.Aliases) == 0 {
			token.Name = keywordName(tokenType.Keyword)
		}
	case tokens.KindRegexp:
		token.Regexp = tokenType.Regexp
	}
	if command := member.Command; command != nil {
		token.Command = command.Kind
		if command.Target != nil {
			token.Target = command.Target.Name
		}
	}
	return token
}
