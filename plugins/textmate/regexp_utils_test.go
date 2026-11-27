// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"regexp"
	"strings"
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
		{"text anchors become line anchors", `^a$`, `^a$`},
		{"explicit text anchors become line anchors", `\Aa\z`, `^a$`},
		{"line anchors", `(?m)^a$`, `^a$`},
		{"control characters", `\x00\t\n\x7f`, `\x{0}\t\n\x{7f}`},
		{"non-ASCII", `ü€`, `ü€`},
		{"string with escapes", `"([^"\\]|\\.)*"`, `"([^"\\]|\\.)*"`},
		{"factored common prefix", `/\*[\s\S]*?\*/|//[^\n\r]*`, `/(?:\*[\s\S]*?\*/|/[^\n\r]*)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toOniguruma(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// matchLines simulates how vscode-textmate applies an Oniguruma regex: one
// line at a time, each including its trailing "\n". Oniguruma's ^ and $ always
// match at line boundaries, like Go's ^ and $ in multi-line mode. It returns
// the first match per line, or "" if there is none.
func matchLines(t *testing.T, oniguruma, document string) []string {
	t.Helper()
	re := regexp.MustCompile("(?m)" + oniguruma)
	var result []string
	for _, line := range strings.SplitAfter(document, "\n") {
		if line != "" {
			result = append(result, re.FindString(line))
		}
	}
	return result
}

func TestToOnigurumaAnchorsMatchPerLine(t *testing.T) {
	document := "a = 1 // first\n# second\nb // third\n"
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{"anchored line comment", `//[^\n]*$`, []string{"// first", "", "// third"}},
		{"line comment with explicit end of text", `//[^\n]*\z`, []string{"// first", "", "// third"}},
		{"comment at line start", `^#.*`, []string{"", "# second", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oniguruma, err := toOniguruma(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, matchLines(t, oniguruma, document))
		})
	}
}

func TestToOnigurumaInvalid(t *testing.T) {
	_, err := toOniguruma(`(`)
	require.Error(t, err)
}

func TestSplitDelimiters(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []delimiters
	}{
		{"line comment", `//[^\n\r]*`, []delimiters{
			{Begin: `//`, HasBody: true},
		}},
		{"shell comment", `#[^\n\r]*`, []delimiters{
			{Begin: `#`, HasBody: true},
		}},
		{"block comment", `/\*[\s\S]*?\*/`, []delimiters{
			{Begin: `/\*`, End: `\*/`, HasBody: true},
		}},
		{"combined comments in source order", `/\*[\s\S]*?\*/|//[^\n\r]*`, []delimiters{
			{Begin: `/\*`, End: `\*/`, HasBody: true},
			{Begin: `//`, HasBody: true},
		}},
		{"string with escapes", `"([^"\\]|\\.)*"`, []delimiters{
			{Begin: `"`, End: `"`, HasBody: true},
		}},
		{"two string styles", `"[^"]*"|'[^']*'`, []delimiters{
			{Begin: `"`, End: `"`, HasBody: true},
			{Begin: `'`, End: `'`, HasBody: true},
		}},
		{"optional part is variable", `ab?c*d`, []delimiters{
			{Begin: `a`, End: `d`, HasBody: true},
		}},
		{"plus keeps leading rune", `a+b`, []delimiters{
			{Begin: `a`, End: `b`, HasBody: true},
		}},
		{"leading anchor is part of begin", `^#.*`, []delimiters{
			{Begin: `^#`, HasBody: true},
		}},
		{"trailing anchor alone is no end", `//[^\n]*$`, []delimiters{
			{Begin: `//`, HasBody: true},
		}},
		{"fixed token", `->`, []delimiters{
			{Begin: `->`},
		}},
		{"fixed token with assertion", `\bfoo`, []delimiters{
			{Begin: `\bfoo`},
		}},
		{"no literal delimiters", `[a-z]+`, []delimiters{
			{HasBody: true},
		}},
		{"assertions alone are no delimiters", `\b[a-z]+\b`, []delimiters{
			{HasBody: true},
		}},
		{"case-insensitive begin", `(?i)rem[^\n]*`, []delimiters{
			{Begin: `(?i:REM)`, HasBody: true},
		}},
		{"duplicates are removed", `ax*|ay*`, []delimiters{
			{Begin: `a`, HasBody: true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitDelimiters(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSplitDelimitersErrors(t *testing.T) {
	_, err := splitDelimiters(`(`)
	require.Error(t, err)

	_, err = splitDelimiters(`(?:aa|bb)(?:cc|dd)(?:ee|ff)(?:gg|hh)(?:ii|jj)(?:kk|ll)`)
	require.ErrorIs(t, err, errTooManyAlternatives)
}
