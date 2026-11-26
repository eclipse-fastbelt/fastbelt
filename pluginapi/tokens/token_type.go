// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package tokens

// TokenKind tells which grammar construct produced a [TokenType].
type TokenKind int

const (
	// KindEOF is the built-in end-of-input token type at index 0.
	KindEOF TokenKind = iota
	// KindKeyword is a keyword literal such as "entry", including keywords
	// that a token declaration with keyword content (token A: "a") names.
	KindKeyword
	// KindRegexp is a token declaration with regexp content (token ID: /.../).
	KindRegexp
	// KindGroup is a token group (token group G { ... }), including the
	// operator groups synthesized for infix rules.
	KindGroup
)

// String returns the name of the kind, e.g. "Keyword".
func (k TokenKind) String() string {
	switch k {
	case KindEOF:
		return "EOF"
	case KindKeyword:
		return "Keyword"
	case KindRegexp:
		return "Regexp"
	case KindGroup:
		return "Group"
	default:
		return "TokenKind(?)"
	}
}

// Modifier is a token modifier as written in the grammar. It is shared with
// the AST mirror of the grammar.
type Modifier string

const (
	// ModifierNone means that no modifier was given.
	ModifierNone Modifier = ""
	// ModifierHidden marks tokens that the lexer drops.
	ModifierHidden Modifier = "hidden"
	// ModifierComment marks tokens that the lexer collects as comments.
	ModifierComment Modifier = "comment"
)

// TokenType is one token type of the generated lexer.
//
// There is exactly one TokenType per runtime token index. The indices are
// dense: 0 is EOF, followed by all keywords, all token declarations with
// regexp content and finally all token groups.
//
// # Names
//
// The Name of a keyword is its value without quotes, e.g. "hello" for the
// keyword "hello". Regexp tokens and token groups are named like their
// declaration, e.g. "ID".
//
// A token declaration with keyword content (token A: "a") does not get an
// index of its own; it shares the index of its keyword. In this case Name is
// the name of the declaration ("A") and the keyword value ("a") is listed in
// Aliases, followed by the names of further declarations of the same
// keyword. Declarations are taken top-level first, then those inside token
// modes, each in grammar order. The Kind stays [KindKeyword], and Modifier is
// the first modifier any of the declarations has. Note that the runtime name
// (core.TokenType.Name) of such a shared index is the keyword value.
type TokenType struct {
	// Index is the runtime token index (core.TokenType.Id and the generated
	// *_Idx constant). BuildArtifacts.TokenTypes[Index] is this token type.
	Index int
	// Name is the primary name, see the type documentation.
	Name string
	// Aliases are the other names sharing this index, see the type
	// documentation for their order. It is nil if there are none.
	Aliases []string
	// Kind tells which grammar construct produced the token type.
	Kind TokenKind
	// Keyword is the keyword value without quotes if Kind is [KindKeyword].
	Keyword string
	// Regexp is the regexp source without the surrounding slashes if Kind is
	// [KindRegexp].
	Regexp string
	// Members are the direct members of a token group if Kind is [KindGroup],
	// ordered by index. A nested group appears as one member of kind
	// [KindGroup]; its own members are not repeated.
	Members []*TokenType
	// Modifier is the modifier of the token declaration. Keywords without a
	// declaration and token groups have no modifier. Modifiers that a token
	// mode applies to the token type are not reflected here.
	Modifier Modifier
	// Synthetic reports whether the token type was created by the generator
	// rather than written in the grammar, such as the operator group of an
	// infix rule.
	Synthetic bool
}
