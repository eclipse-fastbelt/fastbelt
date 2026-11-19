// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package grammar

import (
	"context"
	"log"
	"path"
	"strings"
	"sync"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/util/collections"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

// OpenDirectoryLoader loads the directory of every grammar file the client
// opens. All .fb files of a folder form one grammar, so the files next to an
// opened file are needed even if the folder is not part of a workspace
// folder. Subdirectories are not loaded.
//
// Files are read through the registered [workspace.FileSystem]. Each
// directory is read once; later changes to files that are not open in the
// editor are not picked up.
//
// The loader also handles the opposite direction: when a document is closed,
// it is compared with the file system again. A document without a file, such
// as a deleted file or an untitled document, is removed from the workspace,
// and unsaved changes are replaced by the content of the file. Otherwise a
// closed document would keep contributing to the grammar of its folder.
type OpenDirectoryLoader struct {
	sc *service.Container
	// mu guards loaded.
	mu sync.Mutex
	// loaded holds the unencoded URIs of the directories that were read.
	loaded collections.Set[string]
}

var _ server.InitializeParticipant = (*OpenDirectoryLoader)(nil)

// NewOpenDirectoryLoader creates a new [OpenDirectoryLoader]. Once it is
// registered in the service container, it hooks into the
// [server.DocumentSynchronizer] during server initialization.
func NewOpenDirectoryLoader(sc *service.Container) *OpenDirectoryLoader {
	return &OpenDirectoryLoader{sc: sc, loaded: collections.Set[string]{}}
}

// OnServerInitialize implements [server.InitializeParticipant] by subscribing
// to opened and closed documents.
func (l *OpenDirectoryLoader) OnServerInitialize(_ *lsp.ParamInitialize) {
	synchronizer, err := l.sc.Get[server.DocumentSynchronizer]()
	if err != nil {
		return
	}
	synchronizer.OnDidOpen(func(ctx context.Context, event *server.TextDocumentChangeEvent) {
		// The handler runs under the write lock, before the workspace is rebuilt.
		// Its context is cancelled by the next write, but nothing would load
		// the directory later, so the load must not be skipped.
		l.Load(context.WithoutCancel(ctx), core.ParseURI(string(event.Document.URI())))
	})
	synchronizer.OnDidClose(func(_ context.Context, event *server.TextDocumentChangeEvent) {
		// The handler runs in the write that removes the overlay. Reverting the
		// document is a write of its own, which can only be queued from here.
		go l.Revert(core.ParseURI(string(event.Document.URI())))
	})
}

// Revert brings the closed document with the given URI back in line with the
// file system and rebuilds the workspace: the document is removed if its file
// does not exist, and it takes over the content of the file if that differs.
// A document that is open in the editor is left alone. Revert returns once
// the change is applied; the rebuild continues in the background.
func (l *OpenDirectoryLoader) Revert(uri core.URI) {
	workspace.UpdateInLock(context.Background(), l.sc, func(ctx context.Context) ([]textdoc.Handle, []core.URI, func()) {
		// Like loading, reverting must not be skipped when a newer write arrives.
		ctx = context.WithoutCancel(ctx)
		textStore := l.sc.MustGet[textdoc.Store]()
		doc := l.sc.MustGet[workspace.DocumentManager]().Get(uri)
		if doc == nil || textStore.GetOverlay(uri.DocumentURI()) != nil {
			// Not part of the workspace, or opened again in the meantime.
			return nil, nil, nil
		}
		fs, err := l.sc.Get[workspace.FileSystem]()
		if err != nil {
			return nil, nil, nil
		}
		if entry, err := fs.Stat(ctx, uri); err != nil || entry.IsDir {
			textStore.RemoveFile(uri.DocumentURI())
			return nil, []core.URI{uri}, nil
		}
		content, err := fs.ReadFile(ctx, uri)
		if err != nil {
			log.Printf("failed to read file %s: %v", uri.String(), err)
			return nil, nil, nil
		}
		if string(content) == doc.TextDoc.Text(nil) {
			return nil, nil, nil
		}
		file := textdoc.NewFile(uri.DocumentURI(), doc.TextDoc.LanguageID(), 0, string(content))
		textStore.AddFile(file)
		return []textdoc.Handle{file}, nil, nil
	})
}

// Load loads the directory that contains the document with the given URI,
// unless it was loaded before. It must be called under
// [workspace.Lock.Write] before the build starts; the loaded documents are
// built by the next [workspace.DocumentUpdater.Update].
func (l *OpenDirectoryLoader) Load(ctx context.Context, document core.URI) {
	if !strings.HasPrefix(document.Path(), "/") {
		// Not a hierarchical URI, e.g. an untitled document: there is no directory.
		return
	}
	dir := document.WithPath(path.Dir(document.Path()))
	key := dir.StringUnencoded()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loaded.Has(key) {
		return
	}
	if err := l.loadDirectory(ctx, dir); err != nil {
		// Keep the directory unloaded, so that the next opened document retries.
		log.Printf("failed to load directory %s: %v", key, err)
		return
	}
	l.loaded.Add(key)
}

// loadDirectory adds the files directly inside dir that the
// [workspace.IncludeFilter] accepts to the workspace. Files that are already
// loaded are left unchanged, and a file that cannot be read is skipped.
func (l *OpenDirectoryLoader) loadDirectory(ctx context.Context, dir core.URI) error {
	fs, err := l.sc.Get[workspace.FileSystem]()
	if err != nil {
		return err
	}
	filter, err := l.sc.Get[workspace.IncludeFilter]()
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(ctx, dir)
	if err != nil {
		return err
	}
	docs := l.sc.MustGet[workspace.DocumentManager]()
	textStore := l.sc.MustGet[textdoc.Store]()
	for _, entry := range entries {
		if entry.IsDir || docs.Has(entry.URI) {
			continue
		}
		included, languageID := filter.Include(entry)
		if !included {
			continue
		}
		content, err := fs.ReadFile(ctx, entry.URI)
		if err != nil {
			log.Printf("failed to read file %s: %v", entry.URI.String(), err)
			continue
		}
		textDoc := textdoc.NewFile(entry.URI.DocumentURI(), languageID, 0, string(content))
		textStore.AddFile(textDoc)
		docs.Set(core.NewDocument(textDoc))
	}
	return nil
}
