// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package pluginapi

import (
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/artifacts/astinit"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

func TestTokenDeclarationOfZeroArtifactsIsNil(t *testing.T) {
	require.Nil(t, BuildArtifacts{}.TokenDeclaration(&tokens.TokenType{}))
}

func TestInitWiresSetTokenDeclarations(t *testing.T) {
	require.NotNil(t, astinit.SetTokenDeclarations)
	tt := &tokens.TokenType{Index: 1, Name: "ID", Kind: tokens.KindRegexp}
	decl := &ast.TokenDecl{Name: "ID", TokenType: tt}
	var artifacts BuildArtifacts
	astinit.SetTokenDeclarations(&artifacts, map[*tokens.TokenType]ast.TokenDeclaration{tt: decl})
	require.Same(t, decl, artifacts.TokenDeclaration(tt))
	require.Nil(t, artifacts.TokenDeclaration(&tokens.TokenType{}))
}
