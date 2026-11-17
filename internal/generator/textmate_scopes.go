// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"cmp"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
)

// textMateScopes assigns TextMate scope names to tokens.
type textMateScopes struct {
	// Grammar is the lower-cased grammar name. Every scope gets it as suffix,
	// because the rules of the common file are shared between languages.
	Grammar string
	// Overrides maps a token name to a scope (without the grammar suffix).
	// An empty scope means that the token gets no scope. For keywords
	// without a token declaration the name is grammar.KeywordName.
	Overrides map[string]string
}

// scopeOf returns the scope of token, or "" if the token gets no scope. part
// is the begin/end split of the alternative being emitted (see
// SplitDelimiters); it is the zero value for tokens emitted as a single match.
//
// The heuristic, in order:
//   - hidden tokens never get a scope, not even with an override
//   - an override for the token name wins
//   - comment tokens are comment.block if part has an end delimiter, else comment.line
//   - keywords with letters are keyword.control; punctuation is
//     punctuation.terminator / .separator / .section, other symbols keyword.operator
//   - names with a word STRING or STR are string.quoted.<kind>, where kind
//     comes from the begin delimiter
//   - names with a word like INT, NUMBER or FLOAT are constant.numeric
//   - everything else (e.g. ID) gets no scope
func (s textMateScopes) scopeOf(token textMateToken, part Delimiters) string {
	if token.Modifier == "hidden" {
		return ""
	}
	if scope, ok := s.Overrides[token.Name]; ok {
		return s.withSuffix(scope)
	}
	return s.withSuffix(heuristicScope(token, part))
}

func (s textMateScopes) withSuffix(scope string) string {
	if scope == "" || s.Grammar == "" {
		return scope
	}
	return scope + "." + s.Grammar
}

func heuristicScope(token textMateToken, part Delimiters) string {
	if token.Modifier == "comment" {
		if part.HasBody && part.End != "" {
			return "comment.block"
		}
		return "comment.line"
	}
	if token.Keyword != "" {
		return keywordScope(token.Keyword)
	}
	for _, word := range nameWords(token.Name) {
		switch word {
		case "STRING", "STR":
			return "string.quoted." + quoteKind(part.Begin)
		case "INT", "INTEGER", "NUMBER", "NUM", "FLOAT", "DOUBLE", "DECIMAL", "REAL", "HEX":
			return "constant.numeric"
		}
	}
	return ""
}

func keywordScope(keyword string) string {
	switch {
	case strings.ContainsFunc(keyword, unicode.IsLetter):
		return "keyword.control"
	case keyword == ";":
		return "punctuation.terminator"
	case containsOnly(keyword, ",.:"):
		return "punctuation.separator"
	case containsOnly(keyword, "(){}[]"):
		return "punctuation.section"
	default:
		return "keyword.operator"
	}
}

func containsOnly(s, chars string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return !strings.ContainsRune(chars, r) })
}

// quoteKind names a string delimiter as in TextMate's string.quoted.* scopes.
// begin is the Oniguruma regex of the begin delimiter.
func quoteKind(begin string) string {
	switch begin {
	case `"`:
		return "double"
	case "'":
		return "single"
	case "`":
		return "backtick"
	default:
		return "other"
	}
}

// nameWords splits a token name into upper-cased words at underscores,
// digits and lower-to-upper case changes: "ML_COMMENT" → [ML COMMENT],
// "stringLiteral" → [STRING LITERAL].
func nameWords(name string) []string {
	var words []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToUpper(string(current)))
			current = nil
		}
	}
	var prev rune
	for _, r := range name {
		switch {
		case !unicode.IsLetter(r):
			flush()
		case unicode.IsUpper(r) && unicode.IsLower(prev):
			flush()
			current = append(current, r)
		default:
			current = append(current, r)
		}
		prev = r
	}
	flush()
	return words
}

// keywordsPattern returns an Oniguruma regex that matches any of the given
// unquoted keyword values, or "" if there are none.
//
// A keyword gets a \b on each side that starts or ends with a word character,
// so "in" does not match inside "int" but "+" still matches in "a+b".
//
// TextMate takes the first alternative that matches, while the lexer takes
// the longest match. Keywords are therefore ordered so that a keyword always
// comes after the longer keywords it is a prefix of. Keywords with the same
// boundaries share one group where that order allows: \b(?:while|class|if)\b.
func keywordsPattern(keywords []string) string {
	type entry struct {
		value       string
		left, right bool // whether a \b is needed on that side
		length      int
	}
	seen := map[string]bool{}
	var entries []entry
	for _, kw := range keywords {
		if kw == "" || seen[kw] {
			continue
		}
		seen[kw] = true
		runes := []rune(kw)
		entries = append(entries, entry{
			value:  kw,
			left:   isWordRune(runes[0]),
			right:  isWordRune(runes[len(runes)-1]),
			length: len(runes),
		})
	}
	boundaryRank := func(e entry) int {
		rank := 0
		if !e.left {
			rank += 2
		}
		if !e.right {
			rank++
		}
		return rank
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Or(
			cmp.Compare(b.length, a.length),
			cmp.Compare(boundaryRank(a), boundaryRank(b)),
			strings.Compare(a.value, b.value),
		)
	})

	// Each keyword joins the last group with the same boundaries, unless a
	// group after that one holds a longer keyword starting with it.
	type group struct {
		left, right bool
		values      []string
	}
	var groups []*group
	for _, e := range entries {
		var target *group
		for i := len(groups) - 1; i >= 0; i-- {
			g := groups[i]
			if g.left == e.left && g.right == e.right {
				target = g
				break
			}
			if slices.ContainsFunc(g.values, func(v string) bool { return strings.HasPrefix(v, e.value) }) {
				break
			}
		}
		if target == nil {
			target = &group{left: e.left, right: e.right}
			groups = append(groups, target)
		}
		target.values = append(target.values, e.value)
	}

	patterns := make([]string, 0, len(groups))
	for _, g := range groups {
		alternatives := make([]string, 0, len(g.values))
		for _, value := range g.values {
			alternatives = append(alternatives, onigurumaLiteral(value))
		}
		body := strings.Join(alternatives, "|")
		if len(alternatives) > 1 && (g.left || g.right) {
			body = "(?:" + body + ")"
		}
		if g.left {
			body = `\b` + body
		}
		if g.right {
			body += `\b`
		}
		patterns = append(patterns, body)
	}
	return strings.Join(patterns, "|")
}

// isWordRune reports whether r is a word character for Oniguruma's \b, which
// is Unicode-aware in vscode-oniguruma.
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// onigurumaLiteral escapes s as a literal Oniguruma regex.
func onigurumaLiteral(s string) string {
	return onigurumaOf(&syntax.Regexp{Op: syntax.OpLiteral, Rune: []rune(s)})
}
