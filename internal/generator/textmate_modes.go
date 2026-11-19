// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"context"

	"typefox.dev/fastbelt/internal/grammar"
)

// textMateModel describes the token modes of a grammar as the TextMate
// generator needs them. It mirrors the modes of the generated lexer (see
// lexerModes), so highlighting follows the same mode switches as lexing.
type textMateModel struct {
	// Modes in the order of the generated lexer.
	Modes []*textMateMode
	// ModesByName maps a mode name to its mode.
	ModesByName map[string]*textMateMode
	// Languages are index-aligned with the entry rules passed to
	// buildTextMateModel. There is always at least one language.
	Languages []textMateLanguage
}

// textMateLanguage is one language of a grammar.
type textMateLanguage struct {
	// Entry is the name of the entry rule, or "" if no entry rules were given.
	Entry string
	// EntryMode is the name of the mode the language starts in.
	EntryMode string
}

// textMateMode is one token mode.
type textMateMode struct {
	// Name of the mode: the declared name, "default", or for generated
	// per-language modes the name of the entry rule.
	Name string
	// Tokens that are active in this mode, in lexer priority order
	// (keywords before regexp tokens).
	Tokens []textMateToken
}

// PopTokens returns the tokens of m that pop m from the mode stack. TextMate
// uses them as the end pattern of the rules that push m.
func (m *textMateMode) PopTokens() []textMateToken {
	var result []textMateToken
	for _, token := range m.Tokens {
		if token.Command == "pop" {
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
	// declaration it is the generated keyword name (see grammar.KeywordName).
	Name string
	// Keyword is the unquoted keyword value for keywords and token
	// declarations with keyword content, "" for regexp tokens.
	Keyword string
	// Regexp is the regex without slashes for regexp tokens, "" for keywords.
	Regexp string
	// Modifier is the modifier in this mode: "", "hidden" or "comment".
	Modifier string
	// Command is the mode command in this mode: "", "push", "pop" or "mode".
	// Commands whose target mode does not exist are dropped, like the lexer does.
	Command string
	// Target is the target mode name of a "push" or "mode" command.
	Target string
}

// buildTextMateModel collects the token modes of grammr. entryRules and
// entryModes are index-aligned per language, as for GenerateLexer.
func buildTextMateModel(grammr grammar.Grammar, entryRules []grammar.ParserRule, entryModes []string, tokenTypes GenerateTokenTypesResult) textMateModel {
	ctx := context.Background()
	modes, starts := lexerModes(grammr, entryRules, entryModes, tokenTypes)

	declsByIndex := map[int]grammar.TokenDecl{}
	for decl, tokenIndex := range tokenTypes.TokenIndex.ByToken {
		declsByIndex[tokenIndex] = decl
	}
	keywordsByIndex := map[int]grammar.Keyword{}
	for _, keyword := range tokenTypes.Keywords.Keywords {
		keywordsByIndex[tokenTypes.TokenIndex.ByKeyword[keyword.Value()]] = keyword
	}
	modeNames := map[string]bool{}
	nameByVarName := map[string]string{}
	for _, m := range modes {
		modeNames[m.name] = true
		nameByVarName[m.mode.VarName] = m.name
	}

	model := textMateModel{ModesByName: map[string]*textMateMode{}}
	for _, m := range modes {
		mode := &textMateMode{Name: m.name}
		for _, tokenIndex := range m.tokenIndices(tokenTypes) {
			token := textMateToken{TokenIndex: tokenIndex}
			if decl, ok := declsByIndex[tokenIndex]; ok {
				token.Name = decl.Name()
				switch content := decl.Content().(type) {
				case grammar.RegexpTokenContent:
					token.Regexp = grammar.RegexpValue(content.Regexp())
				case grammar.KeywordTokenContent:
					token.Keyword = grammar.KeywordValue(content.Keyword())
				}
			} else if keyword, ok := keywordsByIndex[tokenIndex]; ok {
				token.Name = grammar.KeywordName(keyword)
				token.Keyword = grammar.KeywordValue(keyword)
			}
			usage := m.mode.TokenTypeUsages[tokenIndex]
			token.Modifier = usage.TokenModifier
			if cmd := usage.Command; cmd != nil {
				var target string
				if cmd.Mode() != nil {
					if targetMode := cmd.Mode().Ref(ctx); targetMode != nil {
						target = targetMode.Name()
					}
				}
				switch {
				case cmd.Type() == "pop":
					token.Command = "pop"
				case modeNames[target]:
					token.Command = cmd.Type()
					token.Target = target
				}
			}
			mode.Tokens = append(mode.Tokens, token)
		}
		model.Modes = append(model.Modes, mode)
		model.ModesByName[mode.Name] = mode
	}

	for i, start := range starts {
		language := textMateLanguage{EntryMode: nameByVarName[start]}
		if i < len(entryRules) {
			language.Entry = entryRules[i].Name()
		}
		model.Languages = append(model.Languages, language)
	}
	return model
}
