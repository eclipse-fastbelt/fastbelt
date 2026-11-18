// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/lsp"
)

func record(msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

// A record is sent with its attributes, those of WithAttrs first, and the
// keys of groups are qualified with the group names.
func TestSlogHandlerSendsAttributes(t *testing.T) {
	h := NewSlogHandler(service.NewContainer()).(*SlogHandler)
	assert.Equal(t, "built documents=42 in=3ms", h.message(record("built", slog.Int("documents", 42), slog.Duration("in", 3*time.Millisecond))))
	assert.Equal(t, `failed file="a b.txt" empty=""`, h.message(record("failed", slog.String("file", "a b.txt"), slog.String("empty", ""))))

	withAttrs := h.WithAttrs([]slog.Attr{slog.String("server", "x")}).(*SlogHandler)
	assert.Equal(t, "built server=x documents=1", withAttrs.message(record("built", slog.Int("documents", 1))))
	// WithAttrs does not change the handler it was called on.
	assert.Equal(t, "built", h.message(record("built")))

	grouped := withAttrs.WithGroup("build").WithAttrs([]slog.Attr{slog.Int("round", 2)}).(*SlogHandler)
	assert.Equal(t, "built server=x build.round=2 build.documents=1 build.phase.name=link",
		grouped.message(record("built", slog.Int("documents", 1), slog.Group("phase", slog.String("name", "link")), slog.Group("empty"), slog.Attr{})))
}

// By default, records below Info are not sent; the level is configurable.
func TestSlogHandlerLevel(t *testing.T) {
	ctx := context.Background()
	h := NewSlogHandler(service.NewContainer())
	assert.False(t, h.Enabled(ctx, slog.LevelDebug))
	assert.True(t, h.Enabled(ctx, slog.LevelInfo))
	debug := NewSlogHandlerWithOptions(service.NewContainer(), SlogHandlerOptions{Level: slog.LevelDebug})
	assert.True(t, debug.Enabled(ctx, slog.LevelDebug))
}

// The handler does not take part in initializing a server, where it used to
// make itself the process-wide default logger.
func TestSlogHandlerIsNotTheDefaultLogger(t *testing.T) {
	_, participates := NewSlogHandler(service.NewContainer()).(InitializeParticipant)
	assert.False(t, participates)
}

// Until the client has initialized the server, and while there is no
// connection, records are written to the fallback (stderr) instead of being
// dropped; handlers derived with WithAttrs share the fallback.
func TestSlogHandlerFallsBackBeforeInitialize(t *testing.T) {
	sc := service.NewContainer()
	connection := &Connection{}
	sc.Put(connection)
	sc.Seal()
	h := NewSlogHandler(sc).(*SlogHandler)
	var out bytes.Buffer
	h.fallback.w = &out
	derived := h.WithAttrs([]slog.Attr{slog.String("server", "x")})

	r := record("cannot start", slog.String("reason", "no port"))
	require.NoError(t, derived.Handle(context.Background(), r))
	assert.Equal(t, "INFO cannot start server=x reason=\"no port\"\n", out.String())

	// Initialized, but without a connection: still the fallback.
	out.Reset()
	connection.SetInitialized()
	assert.True(t, connection.Initialized())
	r.Time = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	require.NoError(t, derived.Handle(context.Background(), r))
	assert.Equal(t, "2026/11/16 12:00:00 INFO cannot start server=x reason=\"no port\"\n", out.String())
}

// Keys that could not be told apart from their values are quoted.
func TestSlogHandlerQuotesKeys(t *testing.T) {
	h := NewSlogHandler(service.NewContainer()).(*SlogHandler)
	assert.Equal(t, `built "a b"=1 "x=y"=2`, h.message(record("built", slog.Int("a b", 1), slog.Int("x=y", 2))))
}

func TestSlogHandlerMessageTypes(t *testing.T) {
	assert.Equal(t, lsp.Log, messageType(slog.LevelDebug))
	assert.Equal(t, lsp.Info, messageType(slog.LevelInfo))
	assert.Equal(t, lsp.Warning, messageType(slog.LevelWarn))
	assert.Equal(t, lsp.Error, messageType(slog.LevelError))
	assert.Equal(t, lsp.Error, messageType(slog.LevelError+4))
}
