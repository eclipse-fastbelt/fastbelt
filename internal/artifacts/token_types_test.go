// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/internal/languages/token_groups"
	"typefox.dev/fastbelt/pluginapi/tokens"
	"typefox.dev/fastbelt/test"
)

// tokenTypesOf converts the token types of a grammar the way the build does:
// infix rules are expanded before the token types are computed.
func tokenTypesOf(t *testing.T, text string) []*tokens.TokenType {
	t.Helper()
	f := test.New(t, grammar.CreateServices())
	doc := f.Parse(text).AssertNoErrors()
	g, ok := doc.Document.Root.(grammar.Grammar)
	require.True(t, ok)
	require.NoError(t, grammar.ExpandInfixRules(g))
	tts, err := TokenTypes(g, generator.GenerateTokenTypes(g))
	require.NoError(t, err)
	requireDense(t, tts)
	return tts
}

func tokenTypesOfFile(t *testing.T, path string) []*tokens.TokenType {
	t.Helper()
	text, err := os.ReadFile(path)
	require.NoError(t, err)
	return tokenTypesOf(t, string(text))
}

// requireDense asserts the index invariants every token type list must hold.
func requireDense(t *testing.T, tts []*tokens.TokenType) {
	t.Helper()
	require.NotEmpty(t, tts)
	require.Equal(t, &tokens.TokenType{Index: 0, Name: "EOF", Kind: tokens.KindEOF}, tts[0])
	for i, tt := range tts {
		require.NotNil(t, tt, "index %d", i)
		require.Equal(t, i, tt.Index, "token type %q", tt.Name)
	}
}

func byName(t *testing.T, tts []*tokens.TokenType, name string) *tokens.TokenType {
	t.Helper()
	for _, tt := range tts {
		if tt.Name == name {
			return tt
		}
	}
	require.Failf(t, "token type not found", "%q", name)
	return nil
}

func names(tts []*tokens.TokenType) []string {
	result := make([]string, len(tts))
	for i, tt := range tts {
		result[i] = tt.Name
	}
	return result
}

func TestTokenTypesKeywordsAndRegexps(t *testing.T) {
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model { Name string }
		entry Model: "hello" Name=ID "world";
		token ID: /[a-z]+/;
		hidden token WS: /\s+/;
		comment token SL_COMMENT: /\/\/[^\n]*/;
	`)
	require.Equal(t, []*tokens.TokenType{
		{Index: 0, Name: "EOF", Kind: tokens.KindEOF},
		{Index: 1, Name: "hello", Kind: tokens.KindKeyword, Keyword: "hello"},
		{Index: 2, Name: "world", Kind: tokens.KindKeyword, Keyword: "world"},
		{Index: 3, Name: "ID", Kind: tokens.KindRegexp, Regexp: "[a-z]+"},
		{Index: 4, Name: "WS", Kind: tokens.KindRegexp, Regexp: `\s+`, Modifier: tokens.ModifierHidden},
		{Index: 5, Name: "SL_COMMENT", Kind: tokens.KindRegexp, Regexp: `\/\/[^\n]*`, Modifier: tokens.ModifierComment},
	}, tts)
}

func TestTokenTypesEOFOnly(t *testing.T) {
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model {}
		entry Model: {Model};
	`)
	require.Len(t, tts, 1)
}

func TestTokenTypesGroups(t *testing.T) {
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model { Value string }
		entry Model: "x" Value=Outer;
		token group Inner { ID "a" }
		token group Outer { Inner "b" keywords /^x$/ }
		token ID: /[a-z]+/;
	`)
	inner := byName(t, tts, "Inner")
	outer := byName(t, tts, "Outer")
	for _, group := range []*tokens.TokenType{inner, outer} {
		require.Equal(t, tokens.KindGroup, group.Kind)
		require.Empty(t, group.Keyword)
		require.Empty(t, group.Regexp)
		require.Equal(t, tokens.ModifierNone, group.Modifier)
		require.False(t, group.Synthetic)
		require.True(t, slices.IsSortedFunc(group.Members, func(a, b *tokens.TokenType) int { return a.Index - b.Index }))
	}
	// Members are the shared token type values, not copies.
	require.ElementsMatch(t, []*tokens.TokenType{byName(t, tts, "a"), byName(t, tts, "ID")}, inner.Members)
	// The nested group is a direct member; its own members are not repeated.
	require.ElementsMatch(t, []*tokens.TokenType{byName(t, tts, "b"), byName(t, tts, "x"), inner}, outer.Members)
	// Groups come after all keywords and regexp tokens.
	require.Equal(t, []string{"Inner", "Outer"}, names(tts[len(tts)-2:]))
}

func TestTokenTypesKeywordSelectorMatchesAllKeywords(t *testing.T) {
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model { Value string }
		entry Model: "one" "two" "three" Value=O;
		token group O { keywords /o/ }
	`)
	require.Equal(t, []string{"one", "two"}, names(byName(t, tts, "O").Members))
}

func TestTokenTypesSharedKeywordIndex(t *testing.T) {
	// The validator forbids using "a" inline next to token A: "a", but
	// several declarations may share one keyword.
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model { Value string }
		entry Model: Value=A Value=B Value=G;
		hidden token A: "a";
		token B: "a";
		token group G { B }
	`)
	require.Len(t, tts, 3)
	shared := tts[1]
	require.Equal(t, &tokens.TokenType{
		Index:    1,
		Name:     "A",
		Aliases:  []string{"a", "B"},
		Kind:     tokens.KindKeyword,
		Keyword:  "a",
		Modifier: tokens.ModifierHidden,
	}, shared)
	require.Equal(t, []*tokens.TokenType{shared}, byName(t, tts, "G").Members)
}

func TestTokenTypesKeywordDeclaration(t *testing.T) {
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model { Value string }
		entry Model: Value=A;
		token A: "a";
	`)
	require.Equal(t, []*tokens.TokenType{
		{Index: 0, Name: "EOF", Kind: tokens.KindEOF},
		{Index: 1, Name: "A", Aliases: []string{"a"}, Kind: tokens.KindKeyword, Keyword: "a"},
	}, tts)
}

func TestTokenTypesInfixOperatorGroup(t *testing.T) {
	tts := tokenTypesOfFile(t, "../../examples/arithmetics/arithmetics.fb")
	operators := byName(t, tts, "BinaryExpressionOperator")
	require.Equal(t, tokens.KindGroup, operators.Kind)
	require.True(t, operators.Synthetic)
	require.ElementsMatch(t, []*tokens.TokenType{
		byName(t, tts, "%"),
		byName(t, tts, "^"),
		byName(t, tts, "MultiplicationOp"),
		byName(t, tts, "AdditionOp"),
	}, operators.Members)

	for _, tt := range tts {
		if tt != operators {
			require.False(t, tt.Synthetic, "token type %q", tt.Name)
		}
	}
	require.Equal(t, tokens.ModifierComment, byName(t, tts, "ML_COMMENT").Modifier)
	require.Equal(t, tokens.ModifierHidden, byName(t, tts, "SL_COMMENT").Modifier)
}

// TestTokenTypesMatchGeneratedLanguage checks the artifacts against the
// token types of the generated token_groups language.
func TestTokenTypesMatchGeneratedLanguage(t *testing.T) {
	tts := tokenTypesOfFile(t, "../languages/token_groups/token_groups.fb")
	runtime := []*core.TokenType{
		core.EOF,
		token_groups.Keyword_a,
		token_groups.Keyword_b,
		token_groups.Keyword_c,
		token_groups.Keyword_d,
		token_groups.Keyword_e,
		token_groups.Keyword_f,
		token_groups.Keyword_g,
		token_groups.Keyword_h,
		token_groups.Keyword_nested,
		token_groups.Keyword_one,
		token_groups.Keyword_two,
		token_groups.Token_ID,
		token_groups.Token_INT,
		token_groups.Token_WS,
		token_groups.TokenGroup_Identifier,
		token_groups.TokenGroup_KeywordGroup,
		token_groups.TokenGroup_NestedIdentifier,
		token_groups.TokenGroup_RegexGroup,
	}
	require.Len(t, tts, len(runtime))
	require.Equal(t, token_groups.TokenGroup_RegexGroup_Idx, len(tts)-1)
	for i, rt := range runtime {
		tt := tts[i]
		require.Equal(t, rt.Id, tt.Index)
		require.Equal(t, rt.Name, tt.Name)
		switch rt.Kind {
		case core.TokenKindKeyword:
			require.Equal(t, tokens.KindKeyword, tt.Kind, tt.Name)
		case core.TokenKindGroup:
			require.Equal(t, tokens.KindGroup, tt.Kind, tt.Name)
			require.ElementsMatch(t, runtimeIds(rt.MatchingTokens), leafIndices(tt))
		default:
			if i == 0 {
				require.Equal(t, tokens.KindEOF, tt.Kind)
			} else {
				require.Equal(t, tokens.KindRegexp, tt.Kind, tt.Name)
			}
		}
	}
	require.Equal(t, []string{"nested", "Identifier"}, names(tts[token_groups.TokenGroup_NestedIdentifier_Idx].Members))
}

func runtimeIds(tts []*core.TokenType) []int {
	ids := make([]int, len(tts))
	for i, tt := range tts {
		ids[i] = tt.Id
	}
	return ids
}

// leafIndices returns the indices of all non-group token types a group
// matches, resolving nested groups.
func leafIndices(group *tokens.TokenType) []int {
	seen := map[int]bool{}
	var walk func(*tokens.TokenType)
	walk = func(tt *tokens.TokenType) {
		for _, member := range tt.Members {
			if member.Kind == tokens.KindGroup {
				walk(member)
			} else {
				seen[member.Index] = true
			}
		}
	}
	walk(group)
	result := make([]int, 0, len(seen))
	for index := range seen {
		result = append(result, index)
	}
	return result
}

func TestTokenTypesSharedKeywordIndexModifierFromLaterDeclaration(t *testing.T) {
	tts := tokenTypesOf(t, `
		grammar Test;
		interface Model { Value string }
		entry Model: Value=A Value=B;
		token A: "a";
		comment token B: "a";
	`)
	require.Equal(t, &tokens.TokenType{
		Index:    1,
		Name:     "A",
		Aliases:  []string{"a", "B"},
		Kind:     tokens.KindKeyword,
		Keyword:  "a",
		Modifier: tokens.ModifierComment,
	}, tts[1])
}
