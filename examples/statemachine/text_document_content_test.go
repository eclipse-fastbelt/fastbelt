// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package statemachine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

func TestTextDocumentContentOfBuiltinDocument(t *testing.T) {
	sc := CreateLspServices(func(sc *service.Container) {
		sc.Put(server.NewDefaultTextDocumentContentProvider(sc, "builtin"))
	})
	srv := sc.MustGet[lsp.Server]()
	ctx := context.Background()

	result, err := srv.Initialize(ctx, &lsp.ParamInitialize{})
	require.NoError(t, err)
	require.NotNil(t, result.Capabilities.Workspace)
	require.NotNil(t, result.Capabilities.Workspace.TextDocumentContent)
	assert.Equal(t, []string{"builtin"}, result.Capabilities.Workspace.TextDocumentContent.TextDocumentContentOptions.Schemes)
	// Requests wait until the workspace is loaded.
	require.NoError(t, srv.Initialized(ctx, &lsp.InitializedParams{}))

	const library = "statemachine Library\nevents reset\n"
	uri := "builtin:///library.statemachine"
	sc.MustGet[workspace.DocumentManager]().Set(core.NewDocumentFromString(uri, "statemachine", library))

	content, err := srv.TextDocumentContent(ctx, &lsp.TextDocumentContentParams{URI: uri})
	require.NoError(t, err)
	assert.Equal(t, library, content.Text)

	_, err = srv.TextDocumentContent(ctx, &lsp.TextDocumentContentParams{URI: "builtin:///missing.statemachine"})
	assert.Error(t, err)
	// Only documents of the provider's schemes are served.
	sc.MustGet[workspace.DocumentManager]().Set(core.NewDocumentFromString("file:///ws/a.statemachine", "statemachine", library))
	_, err = srv.TextDocumentContent(ctx, &lsp.TextDocumentContentParams{URI: "file:///ws/a.statemachine"})
	assert.Error(t, err)
}

func TestNoTextDocumentContentCapabilityWithoutProvider(t *testing.T) {
	sc := CreateLspServices(nil)
	result, err := sc.MustGet[lsp.Server]().Initialize(context.Background(), &lsp.ParamInitialize{})
	require.NoError(t, err)
	assert.Nil(t, result.Capabilities.Workspace)
}
