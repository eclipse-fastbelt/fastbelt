// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"iter"
	"slices"
	"sync"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/util/service"
)

// DocumentManager holds all known documents of a workspace in memory.
// All methods are safe for concurrent use.
type DocumentManager interface {
	// Has reports whether a document with the given URI is managed.
	Has(uri core.URI) bool
	// Get returns the document for uri, or nil if it is not managed.
	Get(uri core.URI) *core.Document
	// Set adds or replaces a document keyed by document.URI.
	Set(document *core.Document)
	// All returns every managed document.
	All() iter.Seq[*core.Document]
	// Delete removes the document for uri and returns it, or nil if it was not managed.
	Delete(uri core.URI) *core.Document
	// Clear removes every document from the manager.
	Clear()
}

// DefaultDocumentManager is the default implementation of [DocumentManager].
type DefaultDocumentManager struct {
	sc        *service.Container
	mu        sync.RWMutex
	documents map[string]*core.Document
	// sorted holds the documents ordered by URI, or nil after a change. All
	// rebuilds it on first use.
	sorted []*core.Document
}

// NewDefaultDocumentManager returns a [DocumentManager] backed by a map.
func NewDefaultDocumentManager(sc *service.Container) DocumentManager {
	return &DefaultDocumentManager{
		sc:        sc,
		documents: map[string]*core.Document{},
	}
}

func (d *DefaultDocumentManager) Has(uri core.URI) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.documents[uri.StringUnencoded()]
	return exists
}

func (d *DefaultDocumentManager) Get(uri core.URI) *core.Document {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.documents[uri.StringUnencoded()]
}

func (d *DefaultDocumentManager) Set(document *core.Document) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.documents[document.URI.StringUnencoded()] = document
	d.sorted = nil
}

// All yields the documents ordered by URI. It iterates a snapshot taken when
// All is called; changes during the iteration are not reflected.
func (d *DefaultDocumentManager) All() iter.Seq[*core.Document] {
	docs := d.sortedDocuments()
	return func(yield func(*core.Document) bool) {
		for _, doc := range docs {
			if !yield(doc) {
				return
			}
		}
	}
}

func (d *DefaultDocumentManager) Delete(uri core.URI) *core.Document {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := uri.StringUnencoded()
	doc, ok := d.documents[key]
	if ok {
		delete(d.documents, key)
		d.sorted = nil
	}
	return doc
}

func (d *DefaultDocumentManager) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	clear(d.documents)
	d.sorted = nil
}

// sortedDocuments returns the documents ordered by URI, rebuilding the
// snapshot after a change.
func (d *DefaultDocumentManager) sortedDocuments() []*core.Document {
	d.mu.RLock()
	sorted := d.sorted
	d.mu.RUnlock()
	if sorted != nil {
		return sorted
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sorted == nil {
		keys := make([]string, 0, len(d.documents))
		for key := range d.documents {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		d.sorted = make([]*core.Document, len(keys))
		for i, key := range keys {
			d.sorted[i] = d.documents[key]
		}
	}
	return d.sorted
}
