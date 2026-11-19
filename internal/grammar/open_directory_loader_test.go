// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package grammar

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

// createLoaderServices creates the grammar language server services on top of
// the given file system, with an [OpenDirectoryLoader] registered.
func createLoaderServices(fs workspace.FileSystem) *service.Container {
	sc := service.NewContainer()
	sc.Put(fs)
	SetupServices(sc)
	sc.Put(NewOpenDirectoryLoader(sc))
	// There is no client connection to publish diagnostics to.
	sc.Put(server.NewNoopDiagnosticsPublisher())
	server.SetupDefaultServices(sc)
	sc.Seal()
	return sc
}

func TestOpenDirectoryLoaderLoadsSiblingsOfOpenedDocument(t *testing.T) {
	fs := workspace.NewVirtualFileSystem()
	opened := core.ParseURI("file:///grammar/a.fb")
	sibling := core.ParseURI("file:///grammar/b.fb")
	late := core.ParseURI("file:///grammar/late.fb")
	other := core.ParseURI("file:///grammar/c.other")
	nested := core.ParseURI("file:///grammar/sub/d.fb")
	outside := core.ParseURI("file:///elsewhere/e.fb")
	for _, uri := range []core.URI{opened, sibling, other, nested, outside} {
		require.NoError(t, fs.WriteFile(uri, []byte("grammar OnDisk;")))
	}

	sc := createLoaderServices(fs)
	languageServer := sc.MustGet[lsp.Server]()
	ctx := context.Background()
	_, err := languageServer.Initialize(ctx, &lsp.ParamInitialize{})
	require.NoError(t, err)

	lock := sc.MustGet[workspace.Lock]()
	open := func(uri core.URI) {
		t.Helper()
		require.NoError(t, languageServer.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
			TextDocument: lsp.TextDocumentItem{URI: uri.DocumentURI(), LanguageID: "fastbelt", Version: 1, Text: "grammar InEditor;"},
		}))
		// Wait for the write that runs the handlers and the build.
		require.NoError(t, lock.Read(ctx, func(context.Context) {}))
	}
	open(opened)

	documents := sc.MustGet[workspace.DocumentManager]()
	store := sc.MustGet[textdoc.Store]()
	require.True(t, documents.Has(sibling), "sibling was not loaded")
	assert.Equal(t, "grammar OnDisk;", documents.Get(sibling).TextDoc.Text(nil))
	assert.Equal(t, "fastbelt", documents.Get(sibling).TextDoc.LanguageID())
	assert.NotNil(t, store.GetFile(sibling.DocumentURI()))
	assert.True(t, documents.Get(sibling).State().IsComplete(), "sibling was not built")
	assert.False(t, documents.Has(other), "file of another language was loaded")
	assert.False(t, documents.Has(nested), "subdirectory was loaded")
	assert.False(t, documents.Has(outside), "other directory was loaded")
	// The editor content of the opened document wins over the file.
	assert.Equal(t, "grammar InEditor;", documents.Get(opened).TextDoc.Text(nil))

	// A directory is read only once.
	require.NoError(t, fs.WriteFile(late, []byte("grammar OnDisk;")))
	open(sibling)
	assert.False(t, documents.Has(late))

	// Documents without a directory are ignored.
	open(core.ParseURI("untitled:Untitled-1"))
	open(outside)
	assert.True(t, documents.Has(outside))
}

func TestOpenDirectoryLoaderRetriesFailedDirectory(t *testing.T) {
	fs := workspace.NewVirtualFileSystem()
	sc := createLoaderServices(fs)
	loader := NewOpenDirectoryLoader(sc)
	file := core.ParseURI("file:///grammar/a.fb")
	ctx := context.Background()
	// The directory does not exist yet.
	loader.Load(ctx, file)
	require.NoError(t, fs.WriteFile(file, []byte("grammar Test;")))
	loader.Load(ctx, file)
	assert.True(t, sc.MustGet[workspace.DocumentManager]().Has(file))
}

// A closed document must not keep contributing to the grammar of its folder
// when its file is gone, e.g. a deleted copy of a grammar file.
func TestOpenDirectoryLoaderRevertsClosedDocuments(t *testing.T) {
	const text = "grammar Test;\ninterface Foo { Name string }\nFoo: Name=ID;\ntoken ID: /[a-z]+/;\n"
	fs := workspace.NewVirtualFileSystem()
	existing := core.ParseURI("file:///ws/a.fb")
	copied := core.ParseURI("file:///ws/copy.fb")
	untitled := core.ParseURI("untitled:Untitled-1")
	require.NoError(t, fs.WriteFile(existing, []byte(text)))
	require.NoError(t, fs.WriteFile(copied, []byte(text)))

	sc := createLoaderServices(fs)
	languageServer := sc.MustGet[lsp.Server]()
	ctx := context.Background()
	_, err := languageServer.Initialize(ctx, &lsp.ParamInitialize{})
	require.NoError(t, err)

	lock := sc.MustGet[workspace.Lock]()
	documents := sc.MustGet[workspace.DocumentManager]()
	open := func(uri core.URI, content string) {
		t.Helper()
		require.NoError(t, languageServer.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
			TextDocument: lsp.TextDocumentItem{URI: uri.DocumentURI(), LanguageID: "fastbelt", Version: 1, Text: content},
		}))
	}
	closeDocument := func(uri core.URI) {
		t.Helper()
		require.NoError(t, languageServer.DidClose(ctx, &lsp.DidCloseTextDocumentParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri.DocumentURI()},
		}))
	}
	// eventually waits until check holds for the built workspace. Reverting a
	// closed document is queued in the background.
	eventually := func(check func() bool, message string) {
		t.Helper()
		require.Eventually(t, func() bool {
			result := false
			require.NoError(t, lock.Read(ctx, func(context.Context) { result = check() }))
			return result
		}, 10*time.Second, 5*time.Millisecond, message)
	}
	duplicates := func() int { return len(documents.Get(existing).Diagnostics) }

	open(existing, text)
	open(copied, text)
	eventually(func() bool { return duplicates() > 0 }, "the copy does not clash with the existing file")

	// The copy is deleted, which closes its editor.
	require.NoError(t, fs.Delete(copied))
	closeDocument(copied)
	eventually(func() bool { return !documents.Has(copied) && duplicates() == 0 }, "the deleted copy is still part of the grammar")
	assert.Nil(t, sc.MustGet[textdoc.Store]().Get(copied.DocumentURI()))

	// An untitled document has no file to go back to.
	open(untitled, text)
	eventually(func() bool { return documents.Has(untitled) }, "the untitled document was not added")
	closeDocument(untitled)
	eventually(func() bool { return !documents.Has(untitled) }, "the closed untitled document is still in the workspace")

	// Unsaved changes are dropped in favor of the file.
	closeDocument(existing)
	open(existing, "grammar Unsaved;")
	eventually(func() bool { return documents.Get(existing).TextDoc.Text(nil) == "grammar Unsaved;" }, "the edit was not applied")
	closeDocument(existing)
	eventually(func() bool { return documents.Get(existing).TextDoc.Text(nil) == text }, "the unsaved changes were kept")

	// A document that is opened again right away is left alone.
	open(existing, "grammar Reopened;")
	closeDocument(existing)
	open(existing, "grammar Reopened;")
	sc.MustGet[*OpenDirectoryLoader]().Revert(existing)
	eventually(func() bool { return documents.Get(existing).TextDoc.Text(nil) == "grammar Reopened;" }, "the open document was reverted")
}
