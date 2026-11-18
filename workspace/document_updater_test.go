// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/util/service"
)

func TestDocumentUpdaterNotifiesListenersOfDeletedDocuments(t *testing.T) {
	sc := service.NewContainer()
	textdoc.SetupDefaultServices(sc)
	SetupDefaultServices(sc)
	sc.Seal()

	uri := core.ParseURI("file:///ws/a.txt")
	docs := sc.MustGet[DocumentManager]()
	docs.Set(core.NewDocumentFromString(uri.String(), "plaintext", "a"))

	updater := sc.MustGet[DocumentUpdater]()
	var order []string
	var deletedSeen [][]core.URI
	updater.OnUpdate(func(_ context.Context, changed []core.URI, deleted []core.URI) {
		order = append(order, "first")
		assert.Empty(t, changed)
		// The change is already applied when the listeners run.
		assert.False(t, docs.Has(uri))
		deletedSeen = append(deletedSeen, deleted)
	})
	disposeSecond := updater.OnUpdate(func(context.Context, []core.URI, []core.URI) {
		order = append(order, "second")
	})

	lock := sc.MustGet[Lock]()
	lock.Write(context.Background(), func(ctx context.Context) {
		updater.Update(ctx, nil, []core.URI{uri})
	})
	assert.Equal(t, []string{"first", "second"}, order)
	assert.Equal(t, [][]core.URI{{uri}}, deletedSeen)

	// An update that only finishes a pending build is not reported.
	lock.Write(context.Background(), func(ctx context.Context) {
		updater.Update(ctx, nil, nil)
	})
	assert.Len(t, order, 2)

	// A disposed listener is no longer called; disposing twice is harmless.
	disposeSecond()
	disposeSecond()
	lock.Write(context.Background(), func(ctx context.Context) {
		updater.Update(ctx, nil, []core.URI{uri})
	})
	assert.Equal(t, []string{"first", "second", "first"}, order)
}
