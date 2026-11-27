// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package cmd

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

// warningHandler is the [slog.Handler] of the default build logger. It writes
// one line per record, "Warning: <msg>", followed by the attributes as
// key=value pairs. Records below [slog.LevelWarn] are dropped.
type warningHandler struct {
	mu     *sync.Mutex
	w      io.Writer
	attrs  string // preformatted " key=value" pairs from WithAttrs
	prefix string // group prefix for keys, e.g. "group."
}

func newWarningHandler(w io.Writer) *warningHandler {
	return &warningHandler{mu: &sync.Mutex{}, w: w}
}

func (h *warningHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *warningHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString("Warning: ")
	b.WriteString(r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, h.prefix, a)
		return true
	})
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *warningHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var b strings.Builder
	b.WriteString(h.attrs)
	for _, a := range attrs {
		writeAttr(&b, h.prefix, a)
	}
	c := *h
	c.attrs = b.String()
	return &c
}

func (h *warningHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.prefix = h.prefix + name + "."
	return &c
}

// writeAttr appends " key=value" for a, flattening groups into dotted keys.
func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		groupPrefix := prefix
		if a.Key != "" {
			groupPrefix += a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			writeAttr(b, groupPrefix, ga)
		}
		return
	}
	b.WriteByte(' ')
	b.WriteString(prefix)
	b.WriteString(a.Key)
	b.WriteByte('=')
	value := a.Value.String()
	if value == "" || strings.ContainsAny(value, " =\"\t\n") {
		value = strconv.Quote(value)
	}
	b.WriteString(value)
}
