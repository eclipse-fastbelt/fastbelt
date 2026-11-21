// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"golang.org/x/exp/jsonrpc2"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/lsp"
)

// WorkspaceFolders is populated during the LSP initialize request.
type WorkspaceFolders struct {
	Value []lsp.WorkspaceFolder
}

// Connection holds the active JSON-RPC connection to the LSP client.
// It is assigned by [DefaultBinder.Bind] when the language server is started.
type Connection struct {
	Value *jsonrpc2.Connection
	// initialized is set by SetInitialized.
	initialized atomic.Bool
	// shutdown is set by SetShutdown.
	shutdown atomic.Bool
	// exited is closed by Exit. It is created on first use, so that the zero
	// Connection is ready to use.
	exited     chan struct{}
	exitedInit sync.Once
	exitOnce   sync.Once
}

// ShutdownReceived reports whether the client has sent the shutdown request.
func (c *Connection) ShutdownReceived() bool {
	return c.shutdown.Load()
}

// SetShutdown records that the client has sent the shutdown request. From
// then on, [SlogHandler] and [DefaultDiagnosticsPublisher] no longer send
// messages to the client. [LifecycleHandler] calls it, whatever [lsp.Server]
// is registered; only a [jsonrpc2.Binder] that does not use that handler has
// to call it itself.
func (c *Connection) SetShutdown() {
	c.shutdown.Store(true)
}

// Exit records that the client has sent the exit notification, which makes
// [StartLanguageServer] return. [LifecycleHandler] calls it, whatever
// [lsp.Server] is registered; only a [jsonrpc2.Binder] that does not use that
// handler has to call it itself. Calling it more than once has no further
// effect.
func (c *Connection) Exit() {
	c.exitOnce.Do(func() { close(c.exitedChan()) })
}

// closing reports whether the client has sent the shutdown request or the
// exit notification, after which the server must not send it messages.
func (c *Connection) closing() bool {
	if c.shutdown.Load() {
		return true
	}
	select {
	case <-c.exitedChan():
		return true
	default:
		return false
	}
}

// Exited returns a channel that is closed once the client has sent the exit
// notification.
func (c *Connection) Exited() <-chan struct{} {
	return c.exitedChan()
}

func (c *Connection) exitedChan() chan struct{} {
	c.exitedInit.Do(func() { c.exited = make(chan struct{}) })
	return c.exited
}

// Initialized reports whether the client has sent the initialize request.
// From then on, the server may send the messages that the LSP specification
// allows while the initialize request is pending: window/logMessage (as
// [SlogHandler] does), window/showMessage, window/showMessageRequest and
// telemetry/event. Other requests and notifications must wait until the
// server has responded to initialize. Read Value only after Initialized
// returned true, which orders the read after its assignment by the binder.
func (c *Connection) Initialized() bool {
	return c.initialized.Load()
}

// SetInitialized records that the client has sent the initialize request.
// [DefaultLanguageServer.Initialize] calls it; an [lsp.Server] that handles
// initialize itself, without delegating to it, must call it too.
func (c *Connection) SetInitialized() {
	c.initialized.Store(true)
}

// SetupDefaultServices sets up the default services for the language server.
// It should be called together with [SetupStdioServices] or [SetupWasmServices].
// If any service is already set, it's not overwritten.
func SetupDefaultServices(sc *service.Container) {
	sc.Put(&WorkspaceFolders{})
	sc.Put(&Connection{})
	if !sc.Has[slog.Handler]() {
		sc.Put(NewSlogHandler(sc))
	}
	if !sc.Has[DiagnosticsPublisher]() {
		sc.Put(NewDefaultDiagnosticsPublisher(sc))
	}
	if !sc.Has[lsp.Server]() {
		sc.Put(NewDefaultLanguageServer(sc))
	}
	if !sc.Has[DocumentSynchronizer]() {
		sc.Put(NewDefaultDocumentSynchronizer(sc))
	}
	if !sc.Has[DefinitionProvider]() {
		sc.Put(NewDefaultDefinitionProvider(sc))
	}
	if !sc.Has[ReferencesProvider]() {
		sc.Put(NewDefaultReferencesProvider(sc))
	}
	if !sc.Has[FoldingRangeProvider]() {
		sc.Put(NewDefaultFoldingRangeProvider(sc))
	}
	if !sc.Has[RenameProvider]() {
		sc.Put(NewDefaultRenameProvider(sc))
	}
	if !sc.Has[NameFinder]() {
		sc.Put(NewDefaultNameFinder(sc))
	}
	if !sc.Has[ReferencesFinder]() {
		sc.Put(NewDefaultReferencesFinder(sc))
	}
	if !sc.Has[DocumentSymbolProvider]() {
		sc.Put(NewDefaultDocumentSymbolProvider(sc))
	}
	if !sc.Has[FuzzyMatcher]() {
		sc.Put(NewDefaultFuzzyMatcher())
	}
	if !sc.Has[CompletionProvider]() {
		sc.Put(NewDefaultCompletionProvider(sc))
	}
	if !sc.Has[SnippetRegistry]() {
		sc.Put(NewDefaultSnippetRegistry())
	}
	if !sc.Has[CompletionTriggers]() {
		sc.Put(NewDefaultCompletionTriggers())
	}
	if !sc.Has[CompletionContributor]() {
		sc.Put(NewDefaultCompletionContributor())
	}
	if !sc.Has[SignatureHelpTriggers]() {
		sc.Put(NewDefaultSignatureHelpTriggers())
	}
	if !sc.Has[DocumentHighlightProvider]() {
		sc.Put(NewDefaultDocumentHighlightProvider(sc))
	}
	if !sc.Has[WorkspaceSymbolProvider]() {
		sc.Put(NewDefaultWorkspaceSymbolProvider(sc))
	}
	if !sc.Has[DocumentationProvider]() {
		sc.Put(NewDefaultDocumentationProvider())
	}
	if !sc.Has[HoverProvider]() {
		sc.Put(NewDefaultHoverProvider(sc))
	}
}
