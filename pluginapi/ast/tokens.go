// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import "typefox.dev/fastbelt/pluginapi/tokens"

// AbstractTokenRule is a rule producing a token type: *[TokenDecl] or
// *[TokenGroup].
type AbstractTokenRule interface {
	AbstractRule
	isAbstractTokenRule()
}

// TokenDeclaration is the declaration of a token type: *[TokenDecl] or
// *[TokenGroup]. See
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.TokenDeclaration] for the
// lookup from a token type.
type TokenDeclaration interface {
	Node
	isTokenDeclaration()
}

// TokenDecl is a token declaration: Modifier token Name: Content Command.
// It appears at the top level ([Grammar.Terminals]) or inside a token mode
// ([TokenDeclUsage]).
type TokenDecl struct {
	node
	Name string
	// Modifier is the declared modifier, if any.
	Modifier tokens.Modifier
	Content  TokenContent
	// Command is the declared mode command, or nil if there is none.
	Command *TokenCommand
	// TokenType is the token type the declaration produces. A declaration
	// with keyword content (token A: "a") shares the token type of its
	// keyword, see [tokens.TokenType].
	TokenType *tokens.TokenType
}

// TokenContent is the content of a [TokenDecl]: *[RegexpTokenContent] or
// *[KeywordTokenContent].
type TokenContent interface {
	Node
	isTokenContent()
}

// RegexpTokenContent is a regexp token content such as /[a-z]+/.
type RegexpTokenContent struct {
	node
	// Regexp is the regexp source without the surrounding slashes, like
	// [tokens.TokenType.Regexp].
	Regexp string
}

// KeywordTokenContent is a keyword token content such as "a".
type KeywordTokenContent struct {
	node
	Keyword *Keyword
}

// Keyword is a keyword literal such as "a".
type Keyword struct {
	node
	// Value is the keyword without quotes, like [tokens.TokenType.Keyword].
	Value string
	// TokenType is the token type matching the keyword. Keywords with the
	// same value share one token type.
	TokenType *tokens.TokenType
	// Synthetic reports whether the keyword was created by infix expansion
	// (an operator in the synthetic operator group of an infix rule). Its
	// location is the one of the operator it was copied from.
	Synthetic bool
}

// TokenCommand is a mode command of a token: -> push(Mode), -> pop or
// -> mode(Mode).
type TokenCommand struct {
	node
	Type tokens.CommandKind
	// Mode is the target mode, or nil if there is none (pop).
	Mode *TokenMode
}

// TokenGroup is a token group: token group Name { ... }. It appears at the top
// level ([Grammar.TokenGroups]) or inside a token mode ([TokenGroupUsage]).
type TokenGroup struct {
	node
	Name string
	// TokenRefs are the referenced token declarations and token groups.
	TokenRefs []AbstractTokenRule
	// Keywords are the keyword members.
	Keywords []*Keyword
	// KeywordSelectors are the regexps of the keywords /.../ members, without
	// the surrounding slashes.
	KeywordSelectors []string
	// TokenType is the token type the group produces.
	TokenType *tokens.TokenType
	// Synthetic reports whether the group is the operator group of an infix
	// rule, created by infix expansion. Its location is the one of the infix
	// rule.
	Synthetic bool
}

func (*TokenDecl) isAbstractRule()           {}
func (*TokenDecl) isAbstractTokenRule()      {}
func (*TokenDecl) isTokenDeclaration()       {}
func (*TokenGroup) isAbstractRule()          {}
func (*TokenGroup) isAbstractTokenRule()     {}
func (*TokenGroup) isTokenDeclaration()      {}
func (*RegexpTokenContent) isTokenContent()  {}
func (*KeywordTokenContent) isTokenContent() {}

// TokenMode is a token mode declaration: token mode Name { ... }.
type TokenMode struct {
	node
	Name    string
	Members []TokenModeMember
}

// TokenModeMember is a member of a [TokenMode]: *[TokenDeclUsage],
// *[TokenGroupUsage], *[TokenUsage], *[KeywordUsage] or *[KeywordSelector].
type TokenModeMember interface {
	Node
	isTokenModeMember()
}

// TokenDeclUsage is a token declaration inside a token mode.
type TokenDeclUsage struct {
	node
	Declaration *TokenDecl
}

// TokenGroupUsage is a token group declaration inside a token mode.
type TokenGroupUsage struct {
	node
	Group *TokenGroup
}

// TokenUsage is a reference to a token declaration or token group inside a
// token mode, optionally overriding its modifier and command.
type TokenUsage struct {
	node
	Modifier tokens.Modifier
	TokenRef AbstractTokenRule
	// Command is the mode command, or nil if there is none.
	Command *TokenCommand
}

// KeywordUsage is a keyword inside a token mode, optionally with a modifier
// and command.
type KeywordUsage struct {
	node
	Modifier tokens.Modifier
	Keyword  *Keyword
	// Command is the mode command, or nil if there is none.
	Command *TokenCommand
}

// KeywordSelector is a keywords /.../ member of a token mode.
type KeywordSelector struct {
	node
	// Selector is the regexp without the surrounding slashes.
	Selector string
}

func (*TokenDeclUsage) isTokenModeMember()  {}
func (*TokenGroupUsage) isTokenModeMember() {}
func (*TokenUsage) isTokenModeMember()      {}
func (*KeywordUsage) isTokenModeMember()    {}
func (*KeywordSelector) isTokenModeMember() {}
