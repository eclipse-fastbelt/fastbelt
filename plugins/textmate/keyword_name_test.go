// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeywordName(t *testing.T) {
	tests := map[string]string{
		"hello":  "hello",
		"(":      "LeftParen",
		")":      "RightParen",
		"{":      "LeftBrace",
		"}":      "RightBrace",
		"[":      "LeftBracket",
		"]":      "RightBracket",
		"_":      "Underscore",
		"$":      "Dollar",
		"%":      "Percent",
		"#":      "Hash",
		"@":      "At",
		"!":      "Exclamation",
		"^":      "Caret",
		"&":      "Ampersand",
		"*":      "Asterisk",
		"-":      "Dash",
		"+":      "Plus",
		"=":      "Equals",
		"<":      "LessThan",
		">":      "GreaterThan",
		"?":      "Question",
		"/":      "Slash",
		`\`:      "Backslash",
		"|":      "Pipe",
		"~":      "Tilde",
		"`":      "Backtick",
		".":      "Dot",
		",":      "Comma",
		":":      "Colon",
		";":      "Semicolon",
		" ":      "Space",
		"\t":     "Tab",
		"\n":     "Newline",
		"\r":     "CarriageReturn",
		"#{":     "HashLeftBrace",
		"<=":     "LessThanEquals",
		"a_b":    "aUnderscoreb",
		"'":      "'",
		"\"":     "\"",
		"föö-ß":  "fööDashß",
		"":       "",
		"x\\n.y": "xBackslashnDoty",
	}
	for keyword, want := range tests {
		assert.Equal(t, want, keywordName(keyword), "keyword %q", keyword)
	}
}
