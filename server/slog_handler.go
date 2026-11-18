// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"typefox.dev/fastbelt/util/service"
	"typefox.dev/lsp"
)

// SlogHandler is a [slog.Handler] that sends log messages to the LSP client
// via the connection (window/logMessage). A record is sent as its message
// followed by its attributes as key=value pairs, e.g. "built documents=42";
// the attributes of groups are qualified with the group names, e.g.
// "build.documents=42".
//
// Until the client has sent the initialize request ([Connection.Initialized]),
// and whenever a record cannot be sent (no connection, or the connection is
// closed), the record is written to stderr instead, so that startup errors
// stay visible and nothing is sent before initialize, as the LSP
// specification requires. Every handler of the same container, also one
// wrapped by another handler, switches to the client at initialize.
//
// The handler does not make itself the process-wide default logger; call
// [SetDefaultLogger] from the server's main to send the output of the
// [slog] and [log] packages to the client.
type SlogHandler struct {
	sc    *service.Container
	level slog.Leveler
	// prefix qualifies the keys of attributes: the open groups, each
	// followed by a dot.
	prefix string
	// attrs holds the attributes of WithAttrs, formatted as " key=value".
	attrs []byte
	// fallback is shared with the handlers derived by WithAttrs and WithGroup.
	fallback *slogFallback
}

// slogFallback receives the records that are not sent to the client.
type slogFallback struct {
	mu sync.Mutex
	w  io.Writer
}

// SlogHandlerOptions are the options of a [SlogHandler].
type SlogHandlerOptions struct {
	// Level is the minimum level of the records the handler sends to the
	// client. If nil, it is [slog.LevelInfo].
	Level slog.Leveler
}

// NewSlogHandler creates a new slog handler that writes records of level
// [slog.LevelInfo] and above to the LSP connection.
func NewSlogHandler(sc *service.Container) slog.Handler {
	return NewSlogHandlerWithOptions(sc, SlogHandlerOptions{})
}

// NewSlogHandlerWithOptions creates a new slog handler that writes to the
// LSP connection, configured by options.
func NewSlogHandlerWithOptions(sc *service.Container, options SlogHandlerOptions) slog.Handler {
	level := options.Level
	if level == nil {
		level = slog.LevelInfo
	}
	return &SlogHandler{sc: sc, level: level, fallback: &slogFallback{w: os.Stderr}}
}

// SetDefaultLogger makes the [slog.Handler] registered in sc, by default a
// [SlogHandler], the handler of the process-wide default logger, so that the
// output of the [slog] and [log] packages is sent to the LSP client (until
// the client has initialized the server, to stderr). Call it from the
// server's main, after the container is sealed.
func SetDefaultLogger(sc *service.Container) {
	slog.SetDefault(slog.New(sc.MustGet[slog.Handler]()))
}

func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	msg := h.message(record)
	if err := h.send(ctx, messageType(record.Level), msg); err == nil {
		return nil
	}
	return h.writeFallback(record, msg)
}

// messageType returns the LSP message type of a record level.
func messageType(level slog.Level) lsp.MessageType {
	switch {
	case level < slog.LevelInfo:
		return lsp.Log
	case level < slog.LevelWarn:
		return lsp.Info
	case level < slog.LevelError:
		return lsp.Warning
	default:
		return lsp.Error
	}
}

// message returns the text sent for record: its message and attributes.
func (h *SlogHandler) message(record slog.Record) string {
	msg := append([]byte(record.Message), h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		msg = appendAttr(msg, h.prefix, attr)
		return true
	})
	return string(msg)
}

// WithAttrs returns a handler that adds attrs to every record it sends.
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.attrs = slices.Clip(h.attrs)
	for _, attr := range attrs {
		clone.attrs = appendAttr(clone.attrs, h.prefix, attr)
	}
	return &clone
}

// WithGroup returns a handler that qualifies the keys of the attributes
// added after it with name.
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.prefix = h.prefix + name + "."
	return &clone
}

// appendAttr appends attr as " key=value" to buf, the key qualified with
// prefix; a group appends its attributes. Empty attributes and groups are
// left out, as the slog handler rules require.
func appendAttr(buf []byte, prefix string, attr slog.Attr) []byte {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return buf
	}
	if attr.Value.Kind() == slog.KindGroup {
		if attr.Key != "" {
			prefix += attr.Key + "."
		}
		for _, member := range attr.Value.Group() {
			buf = appendAttr(buf, prefix, member)
		}
		return buf
	}
	buf = append(buf, ' ')
	buf = appendQuoted(buf, prefix+attr.Key)
	buf = append(buf, '=')
	return appendQuoted(buf, attr.Value.String())
}

// appendQuoted appends s to buf, quoted if it is empty or contains spaces,
// '=', '"' or unprintable runes, so that keys and values can be told apart.
func appendQuoted(buf []byte, s string) []byte {
	if s == "" || strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '=' || r == '"' || !unicode.IsPrint(r)
	}) >= 0 {
		return strconv.AppendQuote(buf, s)
	}
	return append(buf, s...)
}

var errNoClient = errors.New("no initialized connection to the client")

// send sends msg to the client, once the client has initialized the server.
func (h *SlogHandler) send(ctx context.Context, level lsp.MessageType, msg string) error {
	conn, err := h.sc.Get[*Connection]()
	if err != nil {
		return err
	}
	if !conn.Initialized() || conn.Value == nil {
		return errNoClient
	}
	lspConn := lsp.ClientDispatcher(conn.Value)
	return lspConn.LogMessage(ctx, &lsp.LogMessageParams{
		Type:    level,
		Message: msg,
	})
}

// writeFallback writes record, formatted as msg, as one line to the fallback
// writer: time (if set), level and message.
func (h *SlogHandler) writeFallback(record slog.Record, msg string) error {
	prefix := record.Level.String()
	if !record.Time.IsZero() {
		prefix = record.Time.Format("2006/01/02 15:04:05") + " " + prefix
	}
	h.fallback.mu.Lock()
	defer h.fallback.mu.Unlock()
	_, err := fmt.Fprintf(h.fallback.w, "%s %s\n", prefix, msg)
	return err
}
