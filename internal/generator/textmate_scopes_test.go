// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScopeOfHeuristic(t *testing.T) {
	scopes := textMateScopes{Grammar: "test"}
	block := Delimiters{Begin: `/\*`, End: `\*/`, HasBody: true}
	line := Delimiters{Begin: `//`, HasBody: true}
	cases := []struct {
		name  string
		token textMateToken
		part  Delimiters
		want  string
	}{
		{"block comment", textMateToken{Name: "ML_COMMENT", Regexp: `/\*[\s\S]*?\*/`, Modifier: "comment"}, block, "comment.block.test"},
		{"line comment", textMateToken{Name: "SL_COMMENT", Regexp: `//[^\n]*`, Modifier: "comment"}, line, "comment.line.test"},
		{"comment without split", textMateToken{Name: "C", Regexp: `#.*`, Modifier: "comment"}, Delimiters{}, "comment.line.test"},
		{"hidden", textMateToken{Name: "WS", Regexp: `\s+`, Modifier: "hidden"}, Delimiters{}, ""},
		{"word keyword", textMateToken{Name: "while", Keyword: "while"}, Delimiters{}, "keyword.control.test"},
		{"keyword token", textMateToken{Name: "ARROW_KW", Keyword: "goto"}, Delimiters{}, "keyword.control.test"},
		{"semicolon", textMateToken{Name: "Semicolon", Keyword: ";"}, Delimiters{}, "punctuation.terminator.test"},
		{"comma", textMateToken{Name: "Comma", Keyword: ","}, Delimiters{}, "punctuation.separator.test"},
		{"double colon", textMateToken{Name: "ColonColon", Keyword: "::"}, Delimiters{}, "punctuation.separator.test"},
		{"brace", textMateToken{Name: "LeftBrace", Keyword: "{"}, Delimiters{}, "punctuation.section.test"},
		{"operator", textMateToken{Name: "PlusEquals", Keyword: "+="}, Delimiters{}, "keyword.operator.test"},
		{"double string", textMateToken{Name: "STRING", Regexp: `"[^"]*"`}, Delimiters{Begin: `"`, End: `"`, HasBody: true}, "string.quoted.double.test"},
		{"single string", textMateToken{Name: "StringLiteral", Regexp: `'[^']*'`}, Delimiters{Begin: `'`, End: `'`, HasBody: true}, "string.quoted.single.test"},
		{"backtick string", textMateToken{Name: "RAW_STR", Regexp: "`[^`]*`"}, Delimiters{Begin: "`", End: "`", HasBody: true}, "string.quoted.backtick.test"},
		{"other string", textMateToken{Name: "STRING", Regexp: `[a-z]+`}, Delimiters{}, "string.quoted.other.test"},
		{"int", textMateToken{Name: "INT", Regexp: `[0-9]+`}, Delimiters{}, "constant.numeric.test"},
		{"number camel case", textMateToken{Name: "decimalNumber", Regexp: `[0-9]+\.[0-9]+`}, Delimiters{}, "constant.numeric.test"},
		{"float", textMateToken{Name: "FLOAT_LIT", Regexp: `[0-9]+\.[0-9]+`}, Delimiters{}, "constant.numeric.test"},
		{"no substring match", textMateToken{Name: "POINT", Regexp: `\.`}, Delimiters{}, ""},
		{"id", textMateToken{Name: "ID", Regexp: `[a-z]+`}, Delimiters{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, scopes.scopeOf(c.token, c.part))
		})
	}
}

func TestScopeOfOverrides(t *testing.T) {
	scopes := textMateScopes{Grammar: "test", Overrides: map[string]string{
		"ID":        "variable.other",
		"INT":       "",
		"Semicolon": "punctuation.other",
		"WS":        "invalid",
	}}
	require.Equal(t, "variable.other.test", scopes.scopeOf(textMateToken{Name: "ID", Regexp: `[a-z]+`}, Delimiters{}))
	require.Equal(t, "", scopes.scopeOf(textMateToken{Name: "INT", Regexp: `[0-9]+`}, Delimiters{}))
	require.Equal(t, "punctuation.other.test", scopes.scopeOf(textMateToken{Name: "Semicolon", Keyword: ";"}, Delimiters{}))
	// Hidden tokens get a scope only through an override.
	require.Equal(t, "invalid.test", scopes.scopeOf(textMateToken{Name: "WS", Regexp: `\s+`, Modifier: "hidden"}, Delimiters{}))
	require.Equal(t, "", scopes.scopeOf(textMateToken{Name: "SPACE", Regexp: `\s+`, Modifier: "hidden"}, Delimiters{}))
}

func TestScopeOfWithoutGrammarSuffix(t *testing.T) {
	require.Equal(t, "keyword.control", textMateScopes{}.scopeOf(textMateToken{Name: "if", Keyword: "if"}, Delimiters{}))
}

func TestNameWords(t *testing.T) {
	require.Equal(t, []string{"ML", "COMMENT"}, nameWords("ML_COMMENT"))
	require.Equal(t, []string{"STRING", "LITERAL"}, nameWords("stringLiteral"))
	require.Equal(t, []string{"INT"}, nameWords("INT32"))
	require.Equal(t, []string{"ID"}, nameWords("ID"))
	require.Nil(t, nameWords("_"))
}

func TestKeywordsMatch(t *testing.T) {
	cases := []struct {
		name     string
		keywords []string
		want     string
	}{
		{"empty", nil, ""},
		{"single word", []string{"if"}, `\bif\b`},
		{"words longest first", []string{"if", "while", "in", "int"}, `\b(?:while|int|if|in)\b`},
		{"punctuation without boundaries", []string{"+", "+=", ";"}, `\+=|\+|;`},
		{"escaping", []string{"(", ".", "*", "&&", "|"}, `&&|\(|\*|\.|\|`},
		{"word then punctuation", []string{"a-"}, `\ba-`},
		{"punctuation then word", []string{"@foo"}, `@foo\b`},
		{"duplicates and empty", []string{"if", "", "if"}, `\bif\b`},
		{"unicode word", []string{"für"}, `\bfür\b`},
		{"mixed groups", []string{"class", "{", "if", "==", "@id"}, `\b(?:class|if)\b|@id\b|==|\{`},
		// "if" must stay after "if(", so it cannot join the word group before it.
		{"prefix across groups", []string{"while", "if", "if("}, `\bwhile\b|\bif\(|\bif\b`},
		{"prefix across punctuation groups", []string{"-", "->x", "--"}, `->x\b|--|-`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			keywords := make([]scopedKeyword, 0, len(c.keywords))
			for _, value := range c.keywords {
				keywords = append(keywords, scopedKeyword{Value: value, Scope: "s"})
			}
			match := keywordsMatch(keywords)
			require.Equal(t, c.want, match.Match)
			require.Nil(t, match.Captures)
			if c.want != "" {
				require.Equal(t, "s", match.Name)
			}
		})
	}
}

func TestKeywordsMatchWithScopes(t *testing.T) {
	match := keywordsMatch([]scopedKeyword{
		{Value: ":", Scope: "punctuation"},
		{Value: ":=", Scope: "operator"},
		{Value: "if", Scope: "control"},
		{Value: "while", Scope: "control"},
		{Value: "hidden"},
	})
	// ":=" stays before ":" although they have different scopes.
	require.Equal(t, `\bhidden\b|(\b(?:while|if)\b)|(:=)|(:)`, match.Match)
	require.Equal(t, "", match.Name)
	require.Equal(t, Captures{
		"1": {Name: "control"},
		"2": {Name: "operator"},
		"3": {Name: "punctuation"},
	}, match.Captures)
}

func TestKeywordRegex(t *testing.T) {
	require.Equal(t, `\bbegin\b`, keywordRegex("begin"))
	require.Equal(t, `#\{`, keywordRegex("#{"))
}
