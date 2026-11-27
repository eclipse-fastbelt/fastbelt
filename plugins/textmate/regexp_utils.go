// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"errors"
	"fmt"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token rule regexes are Go regexes, parsed with [syntax.Perl] flags like the
// lexer does (see internal/regexp). TextMate grammars are evaluated by the
// Oniguruma engine (vscode-oniguruma in VS Code). The functions in this file
// translate between the two by re-emitting the parsed syntax tree, so that
// flags like (?i), (?s), (?m) and (?U) are already resolved by the Go parser.
// Text anchors (\A, \z, and ^, $ without (?m)) become line anchors, because
// TextMate matches one line at a time.

// toOniguruma converts a Go regex (without the surrounding slashes) into an
// equivalent Oniguruma regex for use in a TextMate grammar.
func toOniguruma(pattern string) (string, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", err
	}
	return onigurumaOf(re), nil
}

// delimiters describe one alternative of a token regex split into a fixed
// begin part, a variable body, and a fixed end part. TextMate matches one line
// at a time, so tokens that can span lines (block comments, strings) must be
// expressed as begin/end rules instead of a single match.
type delimiters struct {
	// Begin is an Oniguruma regex for the literal prefix before the first
	// variable part, or "" if the alternative does not start with a literal.
	Begin string
	// End is an Oniguruma regex for the literal suffix after the last
	// variable part, or "" if the alternative does not end with a literal.
	End string
	// HasBody is false if the alternative has no variable part. In that case
	// Begin matches the complete token and End is "".
	HasBody bool
}

// maxDelimiterAlternatives limits how many alternatives splitDelimiters
// expands before giving up.
const maxDelimiterAlternatives = 32

// errTooManyAlternatives is returned by splitDelimiters if a regex expands to
// more than maxDelimiterAlternatives alternatives.
var errTooManyAlternatives = errors.New("regex has too many alternatives to split into begin/end delimiters")

// splitDelimiters splits a Go regex (without the surrounding slashes) into one
// delimiters value per top-level alternative, in source order.
//
// The fixed parts consist of literal characters and zero-width assertions.
// Everything else (character classes, '.', repetitions, optional parts) is
// variable. A leading or trailing "x+" contributes one fixed "x".
func splitDelimiters(pattern string) ([]delimiters, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	paths, err := expandPaths(re)
	if err != nil {
		return nil, err
	}
	result := make([]delimiters, 0, len(paths))
	seen := map[delimiters]bool{}
	for _, path := range paths {
		d := delimitersOf(path)
		if !seen[d] {
			seen[d] = true
			result = append(result, d)
		}
	}
	return result, nil
}

// expandPaths flattens re into a list of alternatives, each being a sequence
// of items. Alternations are expanded unless they are nested in a repetition.
// Literals are split into single-rune items.
func expandPaths(re *syntax.Regexp) ([][]*syntax.Regexp, error) {
	switch re.Op {
	case syntax.OpNoMatch:
		return nil, nil
	case syntax.OpEmptyMatch:
		return [][]*syntax.Regexp{{}}, nil
	case syntax.OpLiteral:
		path := make([]*syntax.Regexp, len(re.Rune))
		for i, r := range re.Rune {
			path[i] = &syntax.Regexp{Op: syntax.OpLiteral, Flags: re.Flags, Rune: []rune{r}}
		}
		return [][]*syntax.Regexp{path}, nil
	case syntax.OpCapture:
		return expandPaths(re.Sub[0])
	case syntax.OpPlus:
		// x+ is x followed by x*, which keeps a fixed leading x.
		star := &syntax.Regexp{Op: syntax.OpStar, Flags: re.Flags, Sub: re.Sub}
		concat := &syntax.Regexp{Op: syntax.OpConcat, Sub: []*syntax.Regexp{re.Sub[0], star}}
		return expandPaths(concat)
	case syntax.OpAlternate:
		var result [][]*syntax.Regexp
		for _, sub := range re.Sub {
			paths, err := expandPaths(sub)
			if err != nil {
				return nil, err
			}
			result = append(result, paths...)
			if len(result) > maxDelimiterAlternatives {
				return nil, errTooManyAlternatives
			}
		}
		return result, nil
	case syntax.OpConcat:
		result := [][]*syntax.Regexp{{}}
		for _, sub := range re.Sub {
			subPaths, err := expandPaths(sub)
			if err != nil {
				return nil, err
			}
			next := make([][]*syntax.Regexp, 0, len(result)*len(subPaths))
			for _, prefix := range result {
				for _, suffix := range subPaths {
					path := make([]*syntax.Regexp, 0, len(prefix)+len(suffix))
					path = append(path, prefix...)
					path = append(path, suffix...)
					next = append(next, path)
				}
			}
			if len(next) > maxDelimiterAlternatives {
				return nil, errTooManyAlternatives
			}
			result = next
		}
		return result, nil
	default:
		return [][]*syntax.Regexp{{re}}, nil
	}
}

func isFixedItem(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText,
		syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return true
	}
	return false
}

func delimitersOf(path []*syntax.Regexp) delimiters {
	first, last := -1, -1
	for i, item := range path {
		if !isFixedItem(item) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return delimiters{Begin: emitItems(path)}
	}
	return delimiters{
		Begin:   emitItems(path[:first]),
		End:     emitItems(path[last+1:]),
		HasBody: true,
	}
}

// emitItems emits a sequence of fixed items. It returns "" if the sequence
// contains no literal, because pure assertions cannot serve as a delimiter.
func emitItems(items []*syntax.Regexp) string {
	hasLiteral := false
	var merged []*syntax.Regexp
	for _, item := range items {
		if item.Op == syntax.OpLiteral {
			hasLiteral = true
			if n := len(merged); n > 0 && merged[n-1].Op == syntax.OpLiteral &&
				merged[n-1].Flags&syntax.FoldCase == item.Flags&syntax.FoldCase {
				merged[n-1].Rune = append(merged[n-1].Rune, item.Rune...)
				continue
			}
			item = &syntax.Regexp{Op: syntax.OpLiteral, Flags: item.Flags, Rune: append([]rune(nil), item.Rune...)}
		}
		merged = append(merged, item)
	}
	if !hasLiteral {
		return ""
	}
	if len(merged) == 1 {
		return onigurumaOf(merged[0])
	}
	return onigurumaOf(&syntax.Regexp{Op: syntax.OpConcat, Sub: merged})
}

// Precedence levels of emitted Oniguruma expressions.
const (
	precAlternate = iota
	precConcat
	precRepeat
	precAtom
)

func onigurumaOf(re *syntax.Regexp) string {
	sb := &strings.Builder{}
	writeOniguruma(sb, re)
	return sb.String()
}

func precedenceOf(re *syntax.Regexp) int {
	switch re.Op {
	case syntax.OpAlternate:
		return precAlternate
	case syntax.OpConcat, syntax.OpEmptyMatch:
		return precConcat
	case syntax.OpLiteral:
		if len(re.Rune) > 1 && re.Flags&syntax.FoldCase == 0 {
			return precConcat
		}
		return precAtom
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		return precRepeat
	}
	return precAtom
}

// writeSub writes re, wrapped in a non-capturing group if its precedence is
// lower than minPrec.
func writeSub(sb *strings.Builder, re *syntax.Regexp, minPrec int) {
	if precedenceOf(re) < minPrec {
		sb.WriteString("(?:")
		writeOniguruma(sb, re)
		sb.WriteString(")")
	} else {
		writeOniguruma(sb, re)
	}
}

func writeOniguruma(sb *strings.Builder, re *syntax.Regexp) {
	switch re.Op {
	case syntax.OpNoMatch:
		sb.WriteString("(?!)")
	case syntax.OpEmptyMatch:
	case syntax.OpLiteral:
		foldCase := re.Flags&syntax.FoldCase != 0
		if foldCase {
			sb.WriteString("(?i:")
		}
		for _, r := range re.Rune {
			writeLiteralRune(sb, r, false)
		}
		if foldCase {
			sb.WriteString(")")
		}
	case syntax.OpCharClass:
		writeCharClass(sb, re.Rune)
	case syntax.OpAnyCharNotNL:
		sb.WriteString(".")
	case syntax.OpAnyChar:
		sb.WriteString(`[\s\S]`)
	// TextMate matches one line at a time, so text anchors become line anchors:
	// \A would only match on the first line of a document, and \z never
	// matches before the trailing "\n" of a line. The WasDollar flag, which
	// tells $ from \z, is deliberately ignored.
	case syntax.OpBeginLine, syntax.OpBeginText:
		sb.WriteString("^")
	case syntax.OpEndLine, syntax.OpEndText:
		sb.WriteString("$")
	case syntax.OpWordBoundary:
		sb.WriteString(`\b`)
	case syntax.OpNoWordBoundary:
		sb.WriteString(`\B`)
	case syntax.OpCapture:
		if re.Name != "" {
			sb.WriteString("(?<")
			sb.WriteString(re.Name)
			sb.WriteByte('>')
		} else {
			sb.WriteString("(")
		}
		writeOniguruma(sb, re.Sub[0])
		sb.WriteString(")")
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		writeSub(sb, re.Sub[0], precAtom)
		switch re.Op {
		case syntax.OpStar:
			sb.WriteString("*")
		case syntax.OpPlus:
			sb.WriteString("+")
		case syntax.OpQuest:
			sb.WriteString("?")
		case syntax.OpRepeat:
			switch {
			case re.Max < 0:
				fmt.Fprintf(sb, "{%d,}", re.Min)
			case re.Min == re.Max:
				fmt.Fprintf(sb, "{%d}", re.Min)
			default:
				fmt.Fprintf(sb, "{%d,%d}", re.Min, re.Max)
			}
		}
		// The Go parser has already applied (?U) to this flag.
		if re.Flags&syntax.NonGreedy != 0 {
			sb.WriteString("?")
		}
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			writeSub(sb, sub, precConcat)
		}
	case syntax.OpAlternate:
		for i, sub := range re.Sub {
			if i > 0 {
				sb.WriteString("|")
			}
			writeSub(sb, sub, precConcat)
		}
	default:
		panic(fmt.Sprintf("unsupported regex operation: %v", re.Op))
	}
}

func writeCharClass(sb *strings.Builder, ranges []rune) {
	if len(ranges) == 0 {
		sb.WriteString("(?!)")
		return
	}
	if len(ranges) == 2 && ranges[0] == 0 && ranges[1] == unicode.MaxRune {
		sb.WriteString(`[\s\S]`)
		return
	}
	// Go stores negated classes as their complement, e.g. [^\n] becomes
	// [\x00-\x09\x0B-\x{10FFFF}]. Emit whichever form is shorter.
	positive := charClassString(ranges, false)
	negative := charClassString(complementRanges(ranges), true)
	if len(negative) < len(positive) {
		sb.WriteString(negative)
	} else {
		sb.WriteString(positive)
	}
}

func charClassString(ranges []rune, negated bool) string {
	sb := &strings.Builder{}
	sb.WriteString("[")
	if negated {
		sb.WriteString("^")
	}
	for i := 0; i+1 < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		writeLiteralRune(sb, lo, true)
		switch hi {
		case lo:
		case lo + 1:
			writeLiteralRune(sb, hi, true)
		default:
			sb.WriteString("-")
			writeLiteralRune(sb, hi, true)
		}
	}
	sb.WriteString("]")
	return sb.String()
}

func complementRanges(ranges []rune) []rune {
	var result []rune
	next := rune(0)
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i] > next {
			result = append(result, next, ranges[i]-1)
		}
		next = ranges[i+1] + 1
	}
	if next <= unicode.MaxRune {
		result = append(result, next, unicode.MaxRune)
	}
	return result
}

// writeLiteralRune writes r so that Oniguruma matches it literally, either
// outside or inside a character class.
func writeLiteralRune(sb *strings.Builder, r rune, inClass bool) {
	switch r {
	case '\t':
		sb.WriteString(`\t`)
		return
	case '\n':
		sb.WriteString(`\n`)
		return
	case '\r':
		sb.WriteString(`\r`)
		return
	case '\f':
		sb.WriteString(`\f`)
		return
	}
	var special string
	if inClass {
		// '&' is escaped because "&&" is class intersection in Oniguruma.
		special = `\[]^-&`
	} else {
		special = `\.^$|?*+()[]{}`
	}
	if r < utf8.RuneSelf && strings.ContainsRune(special, r) {
		sb.WriteByte('\\')
		sb.WriteRune(r)
		return
	}
	if !unicode.IsPrint(r) || r == utf8.RuneError {
		sb.WriteString(`\x{`)
		sb.WriteString(strconv.FormatInt(int64(r), 16))
		sb.WriteByte('}')
		return
	}
	sb.WriteRune(r)
}
