// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package tokens

// CommandKind is the kind of a token command as written in the grammar
// ("-> push(M)", "-> pop", "-> mode(M)"). It is shared with the AST mirror of
// the grammar.
type CommandKind string

const (
	// CommandPush pushes the target mode onto the mode stack.
	CommandPush CommandKind = "push"
	// CommandPop pops the current mode from the mode stack.
	CommandPop CommandKind = "pop"
	// CommandMode replaces the current mode with the target mode.
	CommandMode CommandKind = "mode"
)

// TokenMode is one token mode of the generated lexer.
//
// The modes are the effective ones, exactly as the generated lexer runs them,
// which can differ from the modes declared in the grammar:
//
//   - A grammar without token modes gets one mode named "default" holding all
//     keywords and top-level token declarations.
//   - A grammar without token modes that serves several languages instead
//     gets one [TokenMode.Synthetic] mode per language, named after the
//     language's entry rule. It holds the token types reachable from that
//     rule plus all token types with a modifier or command.
//   - A grammar that declares token modes gets exactly these modes; there is
//     no implicit "default" mode unless the grammar declares one.
type TokenMode struct {
	// Index is the value of the generated TokenMode_* constant.
	// BuildArtifacts.TokenModes[Index] is this mode.
	Index int
	// Name is the name of the mode: the declared name, "default", or the
	// entry rule name for synthetic modes.
	Name string
	// Synthetic reports whether the mode was generated per language instead
	// of being declared in the grammar.
	Synthetic bool
	// Members are the token types the mode matches, in lexer priority order:
	// plain keywords first, then token declarations, each in the order the
	// mode lists them. In a declared mode, a token declaration with keyword
	// content (token A: "a") counts as a token declaration, so a member of
	// kind [KindKeyword] may follow regexp tokens. Token groups are never
	// members; their member token types are.
	Members []*TokenModeMember
}

// TokenModeMember is one token type matched in a [TokenMode].
type TokenModeMember struct {
	// TokenType is the matched token type. It never has kind [KindGroup] or
	// [KindEOF].
	TokenType *TokenType
	// Modifier is the modifier effective in this mode. It may differ from
	// TokenType.Modifier, since a mode can override the modifier of a token.
	Modifier Modifier
	// Command is the mode change triggered by matching the token type, or
	// nil if there is none.
	Command *TokenCommand
}

// TokenCommand changes the token mode when a token type is matched.
type TokenCommand struct {
	// Kind is the kind of the command.
	Kind CommandKind
	// Target is the mode to switch to. It is nil for [CommandPop].
	Target *TokenMode
}
