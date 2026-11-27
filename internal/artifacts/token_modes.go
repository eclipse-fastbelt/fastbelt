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

// TokenModes converts the token modes of the generated lexer, as computed by
// [generator.LexerModes], into plugin artifacts indexed by their mode
// constant. It also returns the entry mode of each language, aligned with
// entryRules. g, entryRules, entryModes and result must be the ones the lexer
// was generated from; tts are the token types converted from result, see
// [TokenTypes].
//
// It fails if a mode refers to a token index outside tts, which would be a
// bug in the generator.
func TokenModes(g grammar.Grammar, entryRules []grammar.ParserRule, entryModes []string, result generator.GenerateTokenTypesResult, tts []*tokens.TokenType) ([]*tokens.TokenMode, []*tokens.TokenMode, error) {
	lexerModes, starts := generator.LexerModes(g, entryRules, entryModes, result)

	// Modes are created first, since commands may target later modes.
	modes := make([]*tokens.TokenMode, len(lexerModes))
	for i, lm := range lexerModes {
		modes[i] = &tokens.TokenMode{Index: i, Name: lm.Name, Synthetic: lm.Synthetic}
	}
	for i, lm := range lexerModes {
		members := make([]*tokens.TokenModeMember, len(lm.Members))
		for j, lmm := range lm.Members {
			if lmm.TokenIndex <= 0 || lmm.TokenIndex >= len(tts) {
				return nil, nil, fmt.Errorf("token mode %s refers to unknown token index %d", lm.Name, lmm.TokenIndex)
			}
			member := &tokens.TokenModeMember{
				TokenType: tts[lmm.TokenIndex],
				Modifier:  tokens.Modifier(lmm.Modifier),
			}
			if lmm.Command != "" {
				member.Command = &tokens.TokenCommand{Kind: tokens.CommandKind(lmm.Command)}
				if lmm.Target >= 0 {
					member.Command.Target = modes[lmm.Target]
				}
			}
			members[j] = member
		}
		modes[i].Members = members
	}

	// LexerModes returns one entry mode even without entry rules, for the
	// lexer to have a start mode; there is no language to report it for.
	entries := make([]*tokens.TokenMode, len(entryRules))
	for i := range entries {
		entries[i] = modes[starts[i]]
	}
	return modes, entries, nil
}
