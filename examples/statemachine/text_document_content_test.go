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
	"typefox.dev/fastbelt/textdoc"
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

	content, err := srv.TextDocumentContent(ctx, &lsp.TextDocumentContentParams{URI: lsp.DocumentURI(uri)})
	require.NoError(t, err)
	assert.Equal(t, library, content.Text)

	_, err = srv.TextDocumentContent(ctx, &lsp.TextDocumentContentParams{URI: "builtin:///missing.statemachine"})
	assert.Error(t, err)
	// Only documents of the provider's schemes are served.
	sc.MustGet[workspace.DocumentManager]().Set(core.NewDocumentFromString("file:///ws/a.statemachine", "statemachine", library))
	_, err = srv.TextDocumentContent(ctx, &lsp.TextDocumentContentParams{URI: "file:///ws/a.statemachine"})
	assert.Error(t, err)
}

// A document served by the content provider only exists in the language
// server, not in the file system. The client opens it like any other document
// when it shows it, and closing it again must not remove it from the workspace.
func TestClosingBuiltinDocumentKeepsIt(t *testing.T) {
	// The server is not initialized: without a client connection, the
	// diagnostics publisher must not be hooked into the document lifecycle.
	sc := CreateLspServices(func(sc *service.Container) {
		sc.Put(server.NewDefaultTextDocumentContentProvider(sc, "builtin"))
	})
	ctx := context.Background()

	const library = "statemachine Library\nevents reset\n"
	uri := lsp.DocumentURI("builtin:///library.statemachine")
	docs := sc.MustGet[workspace.DocumentManager]()
	docs.Set(core.NewDocumentFromString(string(uri), "statemachine", library))

	synchronizer := sc.MustGet[server.DocumentSynchronizer]()
	synchronizer.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{TextDocument: lsp.TextDocumentItem{
		URI: uri, LanguageID: "statemachine", Version: 1, Text: library,
	}})
	synchronizer.DidClose(ctx, &lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})
	// Wait for the write of didClose to finish.
	sc.MustGet[workspace.Lock]().Write(ctx, func(context.Context) {})

	assert.Nil(t, sc.MustGet[textdoc.Store]().GetOverlay(core.NormalizeURI(uri)), "the overlay was not removed")
	assert.True(t, docs.Has(core.ParseURI(string(uri))), "the builtin document was removed from the workspace")
	content, err := sc.MustGet[server.TextDocumentContentProvider]().HandleTextDocumentContentRequest(ctx, &lsp.TextDocumentContentParams{URI: uri})
	require.NoError(t, err)
	assert.Equal(t, library, content.Text)
}

func TestNoTextDocumentContentCapabilityWithoutProvider(t *testing.T) {
	sc := CreateLspServices(nil)
	result, err := sc.MustGet[lsp.Server]().Initialize(context.Background(), &lsp.ParamInitialize{})
	require.NoError(t, err)
	assert.Nil(t, result.Capabilities.Workspace)
}
