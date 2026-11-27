// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"context"
	"slices"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/internal/grammar"
)

// LexerMode is one token mode exactly as the generated lexer registers it.
type LexerMode struct {
	// Index is the value of the generated mode constant and the position of
	// the mode in the result of [LexerModes].
	Index int
	// Name is the display name of the mode.
	Name string
	// VarName is the name of the generated mode constant.
	VarName string
	// Synthetic reports whether the mode was generated for a language
	// instead of being declared in the grammar.
	Synthetic bool
	// Members are the token types of the mode in lexer priority order:
	// [modeTokenTypes.Keywords] first, then [modeTokenTypes.Tokens].
	Members []LexerModeMember
}

// LexerModeMember is one token type registered in a [LexerMode].
type LexerModeMember struct {
	// TokenIndex is the runtime index of the token type.
	TokenIndex int
	// Modifier is "", "hidden" or "comment".
	Modifier string
	// Command is "", "push", "pop" or "mode". Commands whose target mode
	// does not resolve are dropped, as the lexer does not emit them.
	Command string
	// Target is the index of the mode a "push" or "mode" command switches
	// to, or -1.
	Target int
}

// LexerModes returns the token modes of the generated lexer and, per
// language, the index of its entry mode.
//
// Token modes declared in the grammar are emitted as they are; the implicit
// "default" mode is dropped if the grammar declares modes but no "default".
// A grammar without token modes that serves several languages gets one
// synthetic mode per language, named after its entry rule and holding only
// the token types reachable from that rule plus hidden/comment tokens and
// mode switches.
func LexerModes(grammr grammar.Grammar, entryRules []grammar.ParserRule, entryModes []string, tokenTypes GenerateTokenTypesResult) ([]LexerMode, []int) {
	type source struct {
		name      string
		mode      *TokenMode
		synthetic bool
		// reachable restricts the members to these var names. Nil keeps all.
		reachable map[string]bool
	}
	var sources []source
	var starts []int

	declared := len(grammr.TokenModes()) > 0
	if !declared && len(entryRules) > 1 {
		starts = make([]int, len(entryRules))
		for i, entry := range entryRules {
			sources = append(sources, source{
				name:      entry.Name(),
				mode:      tokenTypes.TokenModes["default"],
				synthetic: true,
				reachable: reachableTokenNames(grammr, entry),
			})
			starts[i] = i
		}
	} else {
		hasDefault := !declared
		for _, mode := range grammr.TokenModes() {
			hasDefault = hasDefault || mode.Name() == "default"
		}
		for _, name := range tokenTypes.TokenModeOrder {
			if name == "default" && !hasDefault {
				// The implicit default mode is unreachable when the grammar declares its own modes.
				continue
			}
			sources = append(sources, source{name: name, mode: tokenTypes.TokenModes[name]})
		}
		starts = make([]int, max(1, len(entryRules)))
		for i := range starts {
			name := "default"
			if i < len(entryModes) && entryModes[i] != "" {
				name = entryModes[i]
			}
			// An unknown entry mode is already reported by validation; fall
			// back to the first mode to keep the output compilable.
			starts[i] = max(0, slices.IndexFunc(sources, func(s source) bool { return s.name == name }))
		}
	}

	// Mode commands resolve their target mode by name.
	indexByName := make(map[string]int, len(sources))
	for i, s := range sources {
		indexByName[s.name] = i
	}
	modes := make([]LexerMode, len(sources))
	for i, s := range sources {
		mode := LexerMode{Index: i, Name: s.name, VarName: "TokenMode_" + s.name, Synthetic: s.synthetic}
		for _, tokenIndex := range slices.Concat(s.mode.ModeTokenTypes.Keywords, s.mode.ModeTokenTypes.Tokens) {
			usage, hasUsage := s.mode.TokenTypeUsages[tokenIndex]
			if s.reachable != nil && !s.reachable[tokenTypes.TokenTypes.ByTokenIndex[tokenIndex].VarName] && !hasUsage {
				continue
			}
			mode.Members = append(mode.Members, lexerModeMember(tokenIndex, usage, indexByName))
		}
		modes[i] = mode
	}
	return modes, starts
}

func lexerModeMember(tokenIndex int, usage tokenTypeUsage, indexByName map[string]int) LexerModeMember {
	member := LexerModeMember{TokenIndex: tokenIndex, Modifier: usage.TokenModifier, Target: -1}
	cmd := usage.Command
	if cmd == nil {
		return member
	}
	if cmd.Type() == "pop" {
		member.Command = "pop"
		return member
	}
	if cmd.Mode() == nil {
		return member
	}
	target := cmd.Mode().Ref(context.Background())
	if target == nil {
		return member
	}
	// A push/mode command without a resolvable target mode is reported by
	// validation; drop it instead of emitting a broken lexer.
	if index, ok := indexByName[target.Name()]; ok {
		member.Command = cmd.Type()
		member.Target = index
	}
	return member
}

// reachableTokenNames returns the generated var names (Keyword_*/Token_*) of
// all token types used by the rules reachable from the entry rule.
func reachableTokenNames(grammr grammar.Grammar, entry grammar.ParserRule) map[string]bool {
	ctx := context.Background()
	reachable := map[string]bool{}
	visitedRules := map[string]bool{}
	visitedGroups := map[string]bool{}
	keywords := GetAllKeywords(grammr)

	var visitGroup func(tokenGroup grammar.TokenGroup)
	visitGroup = func(tokenGroup grammar.TokenGroup) {
		if visitedGroups[tokenGroup.Name()] {
			return
		}
		visitedGroups[tokenGroup.Name()] = true
		for _, tokenRef := range tokenGroup.TokenRefs() {
			switch target := tokenRef.Ref(ctx).(type) {
			case grammar.TokenGroup:
				visitGroup(target)
			case grammar.TokenDecl:
				reachable[GeneratedTokenName(target)] = true
			}
		}
		for _, name := range getAllTokenGroupMembers(tokenGroup, keywords) {
			reachable[name] = true
		}
	}

	var visitRule func(rule core.AstNode)
	visitRule = func(rule core.AstNode) {
		for node := range core.AllChildren(rule) {
			switch n := node.(type) {
			case grammar.Keyword:
				reachable[GeneratedTokenName(n)] = true
			case grammar.RuleCall:
				if n.Rule() == nil {
					continue
				}
				switch target := n.Rule().Ref(ctx).(type) {
				case grammar.TokenDecl:
					reachable[GeneratedTokenName(target)] = true
				case grammar.TokenGroup:
					visitGroup(target)
				case grammar.ParserRule, grammar.CompositeRule, grammar.InfixRule:
					// Infix operators and operands are children of the rule node as well.
					if !visitedRules[target.Name()] {
						visitedRules[target.Name()] = true
						visitRule(target)
					}
				}
			}
		}
	}
	visitedRules[entry.Name()] = true
	visitRule(entry)
	return reachable
}
