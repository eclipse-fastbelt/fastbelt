// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"context"
	"sync"
	"testing"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

// mockDocumentUpdater is a test implementation of DocumentUpdater that tracks calls.
type mockDocumentUpdater struct {
	lock        workspace.Lock
	mu          sync.Mutex
	updateCalls []updateCall
}

type updateCall struct {
	changed []textdoc.Handle
	deleted []core.URI
}

func (m *mockDocumentUpdater) Update(ctx context.Context, changed []textdoc.Handle, deleted []core.URI) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls = append(m.updateCalls, updateCall{changed: changed, deleted: deleted})
}

func (m *mockDocumentUpdater) OnUpdate(workspace.DocumentUpdateListener) func() {
	return func() {}
}

func (m *mockDocumentUpdater) getUpdateCalls() []updateCall {
	// The synchronizer returns as soon as the text change is applied and calls
	// Update later in the same write. Wait for that write to finish.
	_ = m.lock.Read(context.Background(), func(context.Context) {})
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]updateCall, len(m.updateCalls))
	copy(result, m.updateCalls)
	return result
}

func (m *mockDocumentUpdater) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls = nil
}

func createTestServicesWithUpdater() (*service.Container, *mockDocumentUpdater) {
	sc := service.NewContainer()
	textdoc.SetupDefaultServices(sc)
	workspace.SetupDefaultServices(sc)
	SetupDefaultServices(sc)

	updater := &mockDocumentUpdater{}
	sc.Override[workspace.DocumentUpdater](updater)

	sc.Seal()
	updater.lock = sc.MustGet[workspace.Lock]()
	return sc, updater
}

func TestTextDocuments_Lifecycle(t *testing.T) {
	sc, updater := createTestServicesWithUpdater()
	ds := NewDefaultDocumentSynchronizer(sc)
	ctx := context.Background()

	// Open a document
	uri := lsp.DocumentURI("file:///test.txt")
	ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: "plaintext",
			Version:    1,
			Text:       "Hello, World!",
		},
	})

	// Verify DocumentUpdater.Update was called for DidOpen
	updateCalls := updater.getUpdateCalls()
	if len(updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(updateCalls))
	}
	if len(updateCalls[0].changed) != 1 {
		t.Fatalf("expected 1 changed document in update call, got %d", len(updateCalls[0].changed))
	}
	if updateCalls[0].changed[0].URI() != uri {
		t.Errorf("expected URI %s, got %s", uri, updateCalls[0].changed[0].URI())
	}

	// Verify document is in collection
	textdocStore := sc.MustGet[textdoc.Store]()
	doc := textdocStore.GetOverlay(uri)
	if doc == nil {
		t.Fatal("document not found in collection")
	}
	if doc.Version() != 1 {
		t.Errorf("expected version 1, got %d", doc.Version())
	}

	// Change the document
	updater.reset()
	ds.DidChange(ctx, &lsp.DidChangeTextDocumentParams{
		TextDocument: lsp.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: lsp.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []lsp.TextDocumentContentChangeEvent{
			{Text: "Hello, Go!"},
		},
	})

	// Verify DocumentUpdater.Update was called for DidChange
	updateCalls = updater.getUpdateCalls()
	if len(updateCalls) != 1 {
		t.Fatalf("expected 1 update call after change, got %d", len(updateCalls))
	}
	if doc.Version() != 2 {
		t.Errorf("expected version 2, got %d", doc.Version())
	}
	if doc.Text(nil) != "Hello, Go!" {
		t.Errorf("expected text 'Hello, Go!', got '%s'", doc.Text(nil))
	}

	// Close the document
	updater.reset()
	ds.DidClose(ctx, &lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	// Verify overlay was removed from store
	if textdocStore.GetOverlay(uri) != nil {
		t.Error("document should have been removed from collection")
	}
}

// closeFixture opens and closes documents against an in-memory file system.
type closeFixture struct {
	sc      *service.Container
	updater *mockDocumentUpdater
	files   *workspace.VirtualFileSystemProvider
	ds      DocumentSynchronizer
}

func newCloseFixture() *closeFixture {
	files := workspace.NewVirtualFileSystemProvider()
	fileSystem := workspace.NewDefaultFileSystem()
	fileSystem.RegisterProvider(core.FileScheme, files)

	sc := service.NewContainer()
	sc.Put[workspace.FileSystem](fileSystem)
	textdoc.SetupDefaultServices(sc)
	workspace.SetupDefaultServices(sc)
	SetupDefaultServices(sc)
	updater := &mockDocumentUpdater{}
	sc.Override[workspace.DocumentUpdater](updater)
	sc.Seal()
	updater.lock = sc.MustGet[workspace.Lock]()
	return &closeFixture{sc: sc, updater: updater, files: files, ds: NewDefaultDocumentSynchronizer(sc)}
}

// openAndClose opens uri, closes it again and returns the update of the close.
func (f *closeFixture) openAndClose(t *testing.T, uri lsp.DocumentURI) updateCall {
	t.Helper()
	ctx := context.Background()
	f.ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{URI: uri, LanguageID: "plaintext", Version: 1, Text: "edited"},
	})
	f.updater.getUpdateCalls() // wait for the write of didOpen
	f.updater.reset()
	f.ds.DidClose(ctx, &lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})
	calls := f.updater.getUpdateCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 update call after close, got %d", len(calls))
	}
	return calls[0]
}

func TestTextDocuments_CloseKeepsExistingFile(t *testing.T) {
	f := newCloseFixture()
	uri := lsp.DocumentURI("file:///ws/test.txt")
	if err := f.files.WriteFile(core.ParseURI(string(uri)), []byte("on disk")); err != nil {
		t.Fatal(err)
	}
	store := f.sc.MustGet[textdoc.Store]()
	store.AddFile(textdoc.NewFile(uri, "plaintext", 0, "on disk"))

	call := f.openAndClose(t, uri)

	if len(call.deleted) != 0 {
		t.Errorf("expected no deleted documents, got %v", call.deleted)
	}
	if store.GetOverlay(uri) != nil {
		t.Error("overlay should have been removed")
	}
	if store.GetFile(uri) == nil {
		t.Error("file of an existing document should be kept in the store")
	}
}

func TestTextDocuments_CloseDeletesMissingFile(t *testing.T) {
	tests := []struct {
		name string
		uri  lsp.DocumentURI
	}{
		{"file deleted while open", "file:///ws/test.txt"},
		{"scheme without provider", "untitled:Untitled-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCloseFixture()
			store := f.sc.MustGet[textdoc.Store]()
			// The file was loaded with the workspace and deleted afterwards.
			store.AddFile(textdoc.NewFile(tt.uri, "plaintext", 0, "on disk"))

			call := f.openAndClose(t, tt.uri)

			if len(call.deleted) != 1 || call.deleted[0].DocumentURI() != tt.uri {
				t.Fatalf("expected %s to be deleted, got %v", tt.uri, call.deleted)
			}
			if len(call.changed) != 0 {
				t.Errorf("expected no changed documents, got %d", len(call.changed))
			}
			if store.Get(tt.uri) != nil {
				t.Error("document should have been removed from the store")
			}
		})
	}
}

func TestTextDocuments_CloseOfUnopenedDocumentDeletesNothing(t *testing.T) {
	f := newCloseFixture()
	uri := lsp.DocumentURI("file:///ws/missing.txt")
	store := f.sc.MustGet[textdoc.Store]()
	store.AddFile(textdoc.NewFile(uri, "plaintext", 0, "stale"))

	f.ds.DidClose(context.Background(), &lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	calls := f.updater.getUpdateCalls()
	if len(calls) != 1 || len(calls[0].deleted) != 0 {
		t.Errorf("expected a single update without deletions, got %v", calls)
	}
	if store.GetFile(uri) == nil {
		t.Error("file should be kept in the store")
	}
}

func TestTextDocuments_MultipleDocuments(t *testing.T) {
	sc, _ := createTestServicesWithUpdater()
	ds := NewDefaultDocumentSynchronizer(sc)
	ctx := context.Background()

	// Open multiple documents
	uris := []lsp.DocumentURI{
		"file:///test1.txt",
		"file:///test2.txt",
		"file:///test3.txt",
	}

	for i, uri := range uris {
		ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
			TextDocument: lsp.TextDocumentItem{
				URI:        uri,
				LanguageID: "plaintext",
				Version:    int32(i + 1),
				Text:       "content",
			},
		})
	}

	// Verify all documents are present
	textdocStore := sc.MustGet[textdoc.Store]()
	all := textdocStore.AllOverlays()
	if len(all) != len(uris) {
		t.Errorf("expected %d documents, got %d", len(uris), len(all))
	}

	keys := textdocStore.KeysOverlays()
	if len(keys) != len(uris) {
		t.Errorf("expected %d keys, got %d", len(uris), len(keys))
	}

	// Verify each document
	for _, uri := range uris {
		doc := textdocStore.GetOverlay(uri)
		if doc == nil {
			t.Errorf("document %s not found", uri)
		}
	}
}

func TestTextDocuments_IncrementalChanges(t *testing.T) {
	sc, _ := createTestServicesWithUpdater()
	ds := NewDefaultDocumentSynchronizer(sc)
	ctx := context.Background()

	uri := lsp.DocumentURI("file:///test.txt")
	ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: "plaintext",
			Version:    1,
			Text:       "Hello, World!",
		},
	})

	// Apply incremental change
	ds.DidChange(ctx, &lsp.DidChangeTextDocumentParams{
		TextDocument: lsp.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: lsp.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []lsp.TextDocumentContentChangeEvent{
			{
				Range: &lsp.Range{
					Start: lsp.Position{Line: 0, Character: 7},
					End:   lsp.Position{Line: 0, Character: 12},
				},
				Text: "Go",
			},
		},
	})

	textdocStore := sc.MustGet[textdoc.Store]()
	doc := textdocStore.GetOverlay(uri)
	if doc == nil {
		t.Fatal("document not found")
	}

	expected := "Hello, Go!"
	if doc.Text(nil) != expected {
		t.Errorf("expected text '%s', got '%s'", expected, doc.Text(nil))
	}
}

func TestTextDocuments_WillSave(t *testing.T) {
	sc, _ := createTestServicesWithUpdater()
	ds := NewDefaultDocumentSynchronizer(sc)
	ctx := context.Background()

	uri := lsp.DocumentURI("file:///test.txt")
	ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: "plaintext",
			Version:    1,
			Text:       "content",
		},
	})

	// Trigger will-save
	ds.WillSave(ctx, &lsp.WillSaveTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Reason:       lsp.Manual,
	})

	// Verify document still exists
	textdocStore := sc.MustGet[textdoc.Store]()
	doc := textdocStore.GetOverlay(uri)
	if doc == nil {
		t.Fatal("document should still exist after WillSave")
	}
}

func TestTextDocuments_WillSaveWaitUntil(t *testing.T) {
	sc, _ := createTestServicesWithUpdater()
	ds := NewDefaultDocumentSynchronizer(sc)
	ctx := context.Background()

	uri := lsp.DocumentURI("file:///test.txt")
	ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: "plaintext",
			Version:    1,
			Text:       "content",
		},
	})

	// Trigger will-save-wait-until
	edits, err := ds.WillSaveWaitUntil(ctx, &lsp.WillSaveTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Reason:       lsp.Manual,
	})
	if err != nil {
		t.Fatalf("WillSaveWaitUntil failed: %v", err)
	}

	// Verify empty edits are returned
	if len(edits) != 0 {
		t.Errorf("expected 0 edits, got %d", len(edits))
	}
}

func TestTextDocuments_DidSave(t *testing.T) {
	sc, _ := createTestServicesWithUpdater()
	ds := NewDefaultDocumentSynchronizer(sc)
	ctx := context.Background()

	uri := lsp.DocumentURI("file:///test.txt")
	ds.DidOpen(ctx, &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: "plaintext",
			Version:    1,
			Text:       "content",
		},
	})

	// Trigger save
	ds.DidSave(ctx, &lsp.DidSaveTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	// Verify document still exists
	textdocStore := sc.MustGet[textdoc.Store]()
	doc := textdocStore.GetOverlay(uri)
	if doc == nil {
		t.Fatal("document should still exist after DidSave")
		return
	}
	if doc.URI() != uri {
		t.Errorf("expected URI %s, got %s", uri, doc.URI())
	}
}
