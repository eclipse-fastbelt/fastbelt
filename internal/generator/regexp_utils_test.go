// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToOniguruma(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{"literal", `abc`, `abc`},
		{"dot excludes newline", `a.b`, `a.b`},
		{"dot with s flag", `(?s)a.b`, `a[\s\S]b`},
		{"escaped metacharacters", `\.\*\+\?\(\)\[\]\{\}\|\^\$\\`, `\.\*\+\?\(\)\[\]\{\}\|\^\$\\`},
		{"P-style named group", `(?P<name>x)`, `(?<name>x)`},
		{"named group", `(?<name>x)`, `(?<name>x)`},
		{"capture", `(a)`, `(a)`},
		{"group under repetition", `(?:ab)*`, `(?:ab)*`},
		{"repetition binds to last rune", `ab*`, `ab*`},
		{"non-greedy", `a*?`, `a*?`},
		{"U flag makes greedy non-greedy", `(?U)a*`, `a*?`},
		{"U flag makes non-greedy greedy", `(?U)a*?`, `a*`},
		{"nested repetition", `(?:a+)+`, `(?:a+)+`},
		{"optional group", `(?:ab)?`, `(?:ab)?`},
		{"bounded repeat", `a{2,3}`, `a{2,3}`},
		{"unbounded repeat", `a{2,}`, `a{2,}`},
		{"exact repeat", `a{2}`, `a{2}`},
		{"negated class", `[^\n\r]*`, `[^\n\r]*`},
		{"negated single rune", `[^a]`, `[^a]`},
		{"range", `[a-z]`, `[a-z]`},
		{"perl class", `\d+`, `[0-9]+`},
		{"any char class", `[\s\S]`, `[\s\S]`},
		{"class escapes", `[-\]\[^&\\]`, `[\&\-\[-\^]`},
		{"case-insensitive literal", `(?i)ab`, `(?i:AB)`},
		{"case-insensitive class", `(?i)[a-c]`, `[A-Ca-c]`},
		{"single rune alternatives become class", `a|b|c`, `[a-c]`},
		{"alternation", `ab|cd`, `ab|cd`},
		{"alternation in concat", `(?:ab|cd)e`, `(?:ab|cd)e`},
		{"word boundaries", `\bfoo\b`, `\bfoo\b`},
		{"text anchors", `^a$`, `\Aa\z`},
		{"line anchors", `(?m)^a$`, `^a$`},
		{"control characters", `\x00\t\n\x7f`, `\x{0}\t\n\x{7f}`},
		{"non-ASCII", `ü€`, `ü€`},
		{"string with escapes", `"([^"\\]|\\.)*"`, `"([^"\\]|\\.)*"`},
		{"factored common prefix", `/\*[\s\S]*?\*/|//[^\n\r]*`, `/(?:\*[\s\S]*?\*/|/[^\n\r]*)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToOniguruma(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestToOnigurumaInvalid(t *testing.T) {
	_, err := ToOniguruma(`(`)
	require.Error(t, err)
}

func TestSplitDelimiters(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []Delimiters
	}{
		{"line comment", `//[^\n\r]*`, []Delimiters{
			{Begin: `//`, HasBody: true},
		}},
		{"shell comment", `#[^\n\r]*`, []Delimiters{
			{Begin: `#`, HasBody: true},
		}},
		{"block comment", `/\*[\s\S]*?\*/`, []Delimiters{
			{Begin: `/\*`, End: `\*/`, HasBody: true},
		}},
		{"combined comments in source order", `/\*[\s\S]*?\*/|//[^\n\r]*`, []Delimiters{
			{Begin: `/\*`, End: `\*/`, HasBody: true},
			{Begin: `//`, HasBody: true},
		}},
		{"string with escapes", `"([^"\\]|\\.)*"`, []Delimiters{
			{Begin: `"`, End: `"`, HasBody: true},
		}},
		{"two string styles", `"[^"]*"|'[^']*'`, []Delimiters{
			{Begin: `"`, End: `"`, HasBody: true},
			{Begin: `'`, End: `'`, HasBody: true},
		}},
		{"optional part is variable", `ab?c*d`, []Delimiters{
			{Begin: `a`, End: `d`, HasBody: true},
		}},
		{"plus keeps leading rune", `a+b`, []Delimiters{
			{Begin: `a`, End: `b`, HasBody: true},
		}},
		{"anchors do not panic", `^#.*`, []Delimiters{
			{Begin: `\A#`, HasBody: true},
		}},
		{"fixed token", `->`, []Delimiters{
			{Begin: `->`},
		}},
		{"fixed token with assertion", `\bfoo`, []Delimiters{
			{Begin: `\bfoo`},
		}},
		{"no literal delimiters", `[a-z]+`, []Delimiters{
			{HasBody: true},
		}},
		{"assertions alone are no delimiters", `\b[a-z]+\b`, []Delimiters{
			{HasBody: true},
		}},
		{"case-insensitive begin", `(?i)rem[^\n]*`, []Delimiters{
			{Begin: `(?i:REM)`, HasBody: true},
		}},
		{"duplicates are removed", `ax*|ay*`, []Delimiters{
			{Begin: `a`, HasBody: true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitDelimiters(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSplitDelimitersErrors(t *testing.T) {
	_, err := SplitDelimiters(`(`)
	require.Error(t, err)

	_, err = SplitDelimiters(`(?:aa|bb)(?:cc|dd)(?:ee|ff)(?:gg|hh)(?:ii|jj)(?:kk|ll)`)
	require.ErrorIs(t, err, ErrTooManyAlternatives)
}
