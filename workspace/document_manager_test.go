// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
)

func newTestDocument(uri string) *core.Document {
	return core.NewDocumentFromString(uri, "test", "")
}

func uris(docs []*core.Document) []string {
	result := make([]string, len(docs))
	for i, doc := range docs {
		result[i] = doc.URI.StringUnencoded()
	}
	return result
}

func TestDocumentManagerAllIsOrderedByURI(t *testing.T) {
	dm := NewDefaultDocumentManager(nil)
	for _, uri := range []string{"file:///ws/c.txt", "file:///ws/a/z.txt", "file:///ws/b.txt", "file:///ws/a.txt"} {
		dm.Set(newTestDocument(uri))
	}
	expected := []string{"file:///ws/a.txt", "file:///ws/a/z.txt", "file:///ws/b.txt", "file:///ws/c.txt"}
	for range 10 {
		assert.Equal(t, expected, uris(slices.Collect(dm.All())))
	}
}

func TestDocumentManagerAllReflectsChanges(t *testing.T) {
	dm := NewDefaultDocumentManager(nil)
	a, b := newTestDocument("file:///ws/a.txt"), newTestDocument("file:///ws/b.txt")
	dm.Set(b)
	assert.Equal(t, []*core.Document{b}, slices.Collect(dm.All()))

	dm.Set(a)
	assert.Equal(t, []*core.Document{a, b}, slices.Collect(dm.All()))

	replacement := newTestDocument("file:///ws/a.txt")
	dm.Set(replacement)
	assert.Equal(t, []*core.Document{replacement, b}, slices.Collect(dm.All()))

	assert.Same(t, replacement, dm.Delete(replacement.URI))
	assert.Equal(t, []*core.Document{b}, slices.Collect(dm.All()))

	assert.Nil(t, dm.Delete(replacement.URI))
	dm.Clear()
	assert.Empty(t, slices.Collect(dm.All()))
}

func TestDocumentManagerAllIteratesASnapshot(t *testing.T) {
	dm := NewDefaultDocumentManager(nil)
	a, b := newTestDocument("file:///ws/a.txt"), newTestDocument("file:///ws/b.txt")
	dm.Set(a)
	seq := dm.All()
	dm.Set(b)
	assert.Equal(t, []*core.Document{a}, slices.Collect(seq))
	assert.Equal(t, []*core.Document{a, b}, slices.Collect(dm.All()))
}

func TestDocumentManagerAllStopsWhenTheLoopBreaks(t *testing.T) {
	dm := NewDefaultDocumentManager(nil)
	dm.Set(newTestDocument("file:///ws/a.txt"))
	dm.Set(newTestDocument("file:///ws/b.txt"))
	count := 0
	for range dm.All() {
		count++
		break
	}
	assert.Equal(t, 1, count)
}

// TestDocumentManagerAllSeesConcurrentChanges checks that a snapshot built
// while documents are added is never kept after the last change: once the
// writers are done, All yields every document.
func TestDocumentManagerAllSeesConcurrentChanges(t *testing.T) {
	dm := NewDefaultDocumentManager(nil)
	const writers, perWriter = 4, 200
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for i := range perWriter {
				dm.Set(newTestDocument(fmt.Sprintf("file:///ws/%d/%03d.txt", w, i)))
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			for range 200 {
				for range dm.All() {
				}
			}
		})
	}
	wg.Wait()
	all := uris(slices.Collect(dm.All()))
	require.Len(t, all, writers*perWriter)
	assert.True(t, slices.IsSorted(all))
}

func BenchmarkDocumentManagerAll(b *testing.B) {
	dm := NewDefaultDocumentManager(nil)
	for i := range 10_000 {
		dm.Set(newTestDocument(fmt.Sprintf("file:///ws/%05d.txt", i)))
	}
	b.ReportAllocs()
	for b.Loop() {
		for range dm.All() {
		}
	}
}
