// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"fmt"

	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// TokenTypes converts the token types computed by
// [generator.GenerateTokenTypes] into plugin artifacts, indexed by their
// runtime token index. g must be the grammar the result was computed from,
// after [grammar.ExpandInfixRules].
//
// It fails if the result does not assign every index from 0 to the highest
// one, which would be a bug in the generator.
func TokenTypes(g grammar.Grammar, result generator.GenerateTokenTypesResult) ([]*tokens.TokenType, error) {
	tts := make([]*tokens.TokenType, result.TokenIndexCount())
	tts[0] = &tokens.TokenType{Index: 0, Name: "EOF", Kind: tokens.KindEOF}

	for _, keyword := range result.Keywords.Keywords {
		index, ok := result.KeywordTokenIndex(keyword)
		if !ok {
			return nil, fmt.Errorf("keyword %s has no token index", keyword.Value())
		}
		value := grammar.KeywordValue(keyword)
		tts[index] = &tokens.TokenType{Index: index, Name: value, Kind: tokens.KindKeyword, Keyword: value}
	}

	// Token declarations with keyword content share the keyword's index. The
	// first declaration becomes the primary name, see [tokens.TokenType].
	declaredKeywords := map[int]bool{}
	for _, decl := range result.TokenDecls.All {
		index, ok := result.TokenDeclTokenIndex(decl)
		if !ok {
			return nil, fmt.Errorf("token %s has no token index", decl.Name())
		}
		switch content := decl.Content().(type) {
		case grammar.KeywordTokenContent:
			tt := tts[index]
			if tt == nil {
				return nil, fmt.Errorf("token %s shares index %d with an unknown keyword", decl.Name(), index)
			}
			if declaredKeywords[index] {
				tt.Aliases = append(tt.Aliases, decl.Name())
			} else {
				declaredKeywords[index] = true
				tt.Aliases = []string{tt.Name}
				tt.Name = decl.Name()
			}
			if tt.Modifier == tokens.ModifierNone {
				tt.Modifier = tokens.Modifier(decl.Modifier())
			}
		case grammar.RegexpTokenContent:
			tts[index] = &tokens.TokenType{
				Index:    index,
				Name:     decl.Name(),
				Kind:     tokens.KindRegexp,
				Regexp:   grammar.RegexpValue(content.Regexp()),
				Modifier: tokens.Modifier(decl.Modifier()),
			}
		}
	}

	// Infix expansion appends its operator groups to the grammar's top-level
	// groups and fails if the name is taken there.
	synthetic := map[string]bool{}
	for _, rule := range g.InfixRules() {
		synthetic[grammar.InfixOperatorGroupName(rule)] = true
	}
	isSynthetic := func(group grammar.TokenGroup) bool {
		_, topLevel := group.Container().(grammar.Grammar)
		return topLevel && synthetic[group.Name()]
	}
	groups := result.TokenGroups.All
	for _, group := range groups {
		index, ok := result.TokenGroupTokenIndex(group)
		if !ok {
			return nil, fmt.Errorf("token group %s has no token index", group.Name())
		}
		tts[index] = &tokens.TokenType{
			Index:     index,
			Name:      group.Name(),
			Kind:      tokens.KindGroup,
			Synthetic: isSynthetic(group),
		}
	}

	for index, tt := range tts {
		if tt == nil {
			return nil, fmt.Errorf("token index %d is not assigned to a token type", index)
		}
	}

	// Members are linked once every group exists, since a group may contain
	// groups declared after it.
	for _, group := range groups {
		index, _ := result.TokenGroupTokenIndex(group)
		memberIndices := result.DirectTokenGroupMemberIndices(group)
		members := make([]*tokens.TokenType, len(memberIndices))
		for i, memberIndex := range memberIndices {
			members[i] = tts[memberIndex]
		}
		tts[index].Members = members
	}
	return tts, nil
}
