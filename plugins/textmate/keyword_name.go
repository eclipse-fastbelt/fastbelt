// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import "strings"

// keywordName converts a keyword value (without quotes) to the identifier the
// Fastbelt generator uses for it in generated Go constant names, e.g.
// "LeftParen" for "(". Letters and digits are kept as they are.
//
// It re-implements the generator's keyword naming, so that repository keys
// and scope override keys stay the same as in the generated Go code.
func keywordName(keyword string) string {
	sb := &strings.Builder{}
	for _, r := range keyword {
		if name, ok := keywordCharNames[r]; ok {
			sb.WriteString(name)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

var keywordCharNames = map[rune]string{
	'(':  "LeftParen",
	')':  "RightParen",
	'{':  "LeftBrace",
	'}':  "RightBrace",
	'[':  "LeftBracket",
	']':  "RightBracket",
	'_':  "Underscore",
	'$':  "Dollar",
	'%':  "Percent",
	'#':  "Hash",
	'@':  "At",
	'!':  "Exclamation",
	'^':  "Caret",
	'&':  "Ampersand",
	'*':  "Asterisk",
	'-':  "Dash",
	'+':  "Plus",
	'=':  "Equals",
	'<':  "LessThan",
	'>':  "GreaterThan",
	'?':  "Question",
	'/':  "Slash",
	'\\': "Backslash",
	'|':  "Pipe",
	'~':  "Tilde",
	'`':  "Backtick",
	'.':  "Dot",
	',':  "Comma",
	':':  "Colon",
	';':  "Semicolon",
	' ':  "Space",
	'\t': "Tab",
	'\n': "Newline",
	'\r': "CarriageReturn",
}
