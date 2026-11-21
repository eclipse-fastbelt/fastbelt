// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/exp/jsonrpc2"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

// testClient is the client's side of the connection. It records the methods
// of the messages the server sends.
type testClient struct {
	conns chan *jsonrpc2.Connection

	mu       sync.Mutex
	received []string
}

func (c *testClient) Bind(_ context.Context, conn *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
	c.conns <- conn
	return jsonrpc2.ConnectionOptions{
		Handler: jsonrpc2.HandlerFunc(func(_ context.Context, req *jsonrpc2.Request) (any, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.received = append(c.received, req.Method)
			return nil, nil
		}),
	}, nil
}

// count returns how many messages with the given method the server has sent.
func (c *testClient) count(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, m := range c.received {
		if m == method {
			n++
		}
	}
	return n
}

// runningServer is a language server started with
// [server.StartLanguageServer] over an in-memory pipe.
type runningServer struct {
	// sc is the server's service container.
	sc *service.Container
	// client sends messages to the server like a language client would.
	client lsp.Server
	// clientConn is the client's side of the connection.
	clientConn *jsonrpc2.Connection
	// received records what the server sent to the client.
	received *testClient
	// connection is the server's Connection service.
	connection *server.Connection
	// result receives the return value of StartLanguageServer.
	result chan error
}

// startServer starts a language server with ctx. The configure functions can
// register services before the defaults are set up.
func startServer(t *testing.T, ctx context.Context, configure ...func(sc *service.Container)) *runningServer {
	t.Helper()

	// StartLanguageServer dials, so the client is the listening side.
	listener, err := jsonrpc2.NetPipe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	client := &testClient{conns: make(chan *jsonrpc2.Connection, 1)}
	if _, err := jsonrpc2.Serve(context.Background(), listener, client); err != nil {
		t.Fatal(err)
	}

	sc := service.NewContainer()
	for _, c := range configure {
		c(sc)
	}
	workspace.SetupDefaultServices(sc)
	sc.Override(workspace.NewNoopInitializer())
	textdoc.SetupDefaultServices(sc)
	server.SetupDefaultServices(sc)
	sc.Put[jsonrpc2.Dialer](listener.Dialer())
	sc.Put[jsonrpc2.Binder](server.NewDefaultAsyncCancelBinder(sc))
	sc.Seal()

	running := &runningServer{
		sc:         sc,
		received:   client,
		connection: sc.MustGet[*server.Connection](),
		result:     make(chan error, 1),
	}
	go func() { running.result <- server.StartLanguageServer(ctx, sc) }()

	select {
	case running.clientConn = <-client.conns:
	case err := <-running.result:
		t.Fatalf("server stopped before connecting: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not connect")
	}
	running.client = lsp.ServerDispatcher(running.clientConn)
	t.Cleanup(func() {
		_ = running.clientConn.Close()
		_ = listener.Close()
	})
	return running
}

// wait returns the result of StartLanguageServer.
func (s *runningServer) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-s.result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("StartLanguageServer did not return")
		return nil
	}
}

// expectRunning fails the test if StartLanguageServer has returned.
func (s *runningServer) expectRunning(t *testing.T) {
	t.Helper()
	select {
	case err := <-s.result:
		t.Fatalf("StartLanguageServer returned early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

// expectReceived waits until the client has received count messages with the
// given method.
func (s *runningServer) expectReceived(t *testing.T, method string, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for s.received.count(method) != count {
		if time.Now().After(deadline) {
			t.Fatalf("client received %d %s messages, want %d", s.received.count(method), method, count)
		}
		time.Sleep(time.Millisecond)
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// Exit after shutdown ends the server without an error, i.e. exit code 0.
func TestExitAfterShutdown(t *testing.T) {
	ctx := testContext(t)
	s := startServer(t, context.Background())
	if _, err := s.client.Initialize(ctx, &lsp.ParamInitialize{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if s.connection.ShutdownReceived() {
		t.Fatal("shutdown recorded before the shutdown request")
	}
	if err := s.client.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if !s.connection.ShutdownReceived() {
		t.Fatal("shutdown not recorded after the shutdown request")
	}
	// Shutdown alone must not end the server: it waits for exit.
	s.expectRunning(t)

	if err := s.client.Exit(ctx); err != nil {
		t.Fatalf("Exit failed: %v", err)
	}
	if err := s.wait(t); err != nil {
		t.Errorf("StartLanguageServer returned %v, want nil", err)
	}
}

// Exit without shutdown ends the server with an error, i.e. exit code 1.
func TestExitWithoutShutdown(t *testing.T) {
	ctx := testContext(t)
	s := startServer(t, context.Background())
	if _, err := s.client.Initialize(ctx, &lsp.ParamInitialize{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if err := s.client.Exit(ctx); err != nil {
		t.Fatalf("Exit failed: %v", err)
	}
	if err := s.wait(t); !errors.Is(err, server.ErrExitWithoutShutdown) {
		t.Errorf("StartLanguageServer returned %v, want ErrExitWithoutShutdown", err)
	}
}

// Exit ends the server even before it was initialized.
func TestExitBeforeInitialize(t *testing.T) {
	s := startServer(t, context.Background())
	if err := s.client.Exit(testContext(t)); err != nil {
		t.Fatalf("Exit failed: %v", err)
	}
	if err := s.wait(t); !errors.Is(err, server.ErrExitWithoutShutdown) {
		t.Errorf("StartLanguageServer returned %v, want ErrExitWithoutShutdown", err)
	}
}

// A shutdown request that the server rejects, because it is the second one,
// does not change the result of the exit that follows.
func TestExitAfterRepeatedShutdown(t *testing.T) {
	ctx := testContext(t)
	s := startServer(t, context.Background())
	if err := s.client.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if err := s.client.Shutdown(ctx); err == nil {
		t.Error("second Shutdown succeeded, want an invalid request error")
	}
	if err := s.client.Exit(ctx); err != nil {
		t.Fatalf("Exit failed: %v", err)
	}
	if err := s.wait(t); err != nil {
		t.Errorf("StartLanguageServer returned %v, want nil", err)
	}
}

// customLifecycleServer handles shutdown and exit itself, without delegating
// to the default language server.
type customLifecycleServer struct {
	lsp.Server
	shutdownErr error
}

func (s *customLifecycleServer) Shutdown(context.Context) error { return s.shutdownErr }
func (s *customLifecycleServer) Exit(context.Context) error     { return nil }

func withCustomLifecycleServer(shutdownErr error) func(sc *service.Container) {
	return func(sc *service.Container) {
		sc.Put[lsp.Server](&customLifecycleServer{
			Server:      server.NewDefaultLanguageServer(sc),
			shutdownErr: shutdownErr,
		})
	}
}

// Shutdown and exit are recorded for a server that handles them itself.
func TestCustomServerLifecycle(t *testing.T) {
	t.Run("exit after shutdown", func(t *testing.T) {
		ctx := testContext(t)
		s := startServer(t, context.Background(), withCustomLifecycleServer(nil))
		if err := s.client.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}
		if err := s.client.Exit(ctx); err != nil {
			t.Fatalf("Exit failed: %v", err)
		}
		if err := s.wait(t); err != nil {
			t.Errorf("StartLanguageServer returned %v, want nil", err)
		}
	})
	t.Run("exit without shutdown", func(t *testing.T) {
		s := startServer(t, context.Background(), withCustomLifecycleServer(nil))
		if err := s.client.Exit(testContext(t)); err != nil {
			t.Fatalf("Exit failed: %v", err)
		}
		if err := s.wait(t); !errors.Is(err, server.ErrExitWithoutShutdown) {
			t.Errorf("StartLanguageServer returned %v, want ErrExitWithoutShutdown", err)
		}
	})
	// A shutdown request that failed does not count as a shutdown.
	t.Run("exit after failed shutdown", func(t *testing.T) {
		ctx := testContext(t)
		s := startServer(t, context.Background(), withCustomLifecycleServer(errors.New("cannot shut down")))
		if err := s.client.Shutdown(ctx); err == nil {
			t.Fatal("Shutdown succeeded, want an error")
		}
		if s.connection.ShutdownReceived() {
			t.Error("failed shutdown was recorded")
		}
		if err := s.client.Exit(ctx); err != nil {
			t.Fatalf("Exit failed: %v", err)
		}
		if err := s.wait(t); !errors.Is(err, server.ErrExitWithoutShutdown) {
			t.Errorf("StartLanguageServer returned %v, want ErrExitWithoutShutdown", err)
		}
	})
}

// A client that closes the stream without shutdown and exit ends the server
// with the error that ended the stream, i.e. exit code 1.
func TestStreamClosedWithoutExit(t *testing.T) {
	s := startServer(t, context.Background())
	if err := s.clientConn.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := s.wait(t); !errors.Is(err, io.EOF) {
		t.Errorf("StartLanguageServer returned %v, want io.EOF", err)
	}
}

// A client that closes the stream after shutdown, instead of sending exit,
// ends the server without an error, i.e. exit code 0.
func TestStreamClosedAfterShutdown(t *testing.T) {
	s := startServer(t, context.Background())
	if err := s.client.Shutdown(testContext(t)); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if err := s.clientConn.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := s.wait(t); err != nil {
		t.Errorf("StartLanguageServer returned %v, want nil", err)
	}
}

// Cancelling the context ends the server with the context's error, although
// the client keeps the stream open.
func TestContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := startServer(t, ctx)
	if _, err := s.client.Initialize(testContext(t), &lsp.ParamInitialize{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	s.expectRunning(t)
	cancel()
	if err := s.wait(t); !errors.Is(err, context.Canceled) {
		t.Errorf("StartLanguageServer returned %v, want context.Canceled", err)
	}
}

// After the shutdown request, the server no longer sends diagnostics or log
// messages to the client.
func TestNoMessagesAfterShutdown(t *testing.T) {
	const (
		publishDiagnostics = "textDocument/publishDiagnostics"
		logMessage         = "window/logMessage"
	)
	ctx := testContext(t)
	s := startServer(t, context.Background())
	if _, err := s.client.Initialize(ctx, &lsp.ParamInitialize{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	publisher := s.sc.MustGet[server.DiagnosticsPublisher]()
	logger := slog.New(s.sc.MustGet[slog.Handler]())
	uri := core.ParseURI("file:///test.txt")

	// While the server runs, both reach the client.
	publisher.ClearDiagnostics(ctx, uri)
	logger.Info("before shutdown")
	s.expectReceived(t, publishDiagnostics, 1)
	s.expectReceived(t, logMessage, 1)

	if err := s.client.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	publisher.ClearDiagnostics(ctx, uri)
	logger.Info("after shutdown")

	// Exit and close, which waits until the client has handled everything
	// the server sent.
	if err := s.client.Exit(ctx); err != nil {
		t.Fatalf("Exit failed: %v", err)
	}
	if err := s.wait(t); err != nil {
		t.Errorf("StartLanguageServer returned %v, want nil", err)
	}
	_ = s.clientConn.Close()
	if n := s.received.count(publishDiagnostics); n != 1 {
		t.Errorf("client received %d %s messages, want 1", n, publishDiagnostics)
	}
	if n := s.received.count(logMessage); n != 1 {
		s.received.mu.Lock()
		received := slices.Clone(s.received.received)
		s.received.mu.Unlock()
		t.Errorf("client received %d %s messages, want 1 (all messages: %v)", n, logMessage, received)
	}
}

// The exit state of a Connection: Exited is closed by the first Exit, and
// further calls are harmless.
func TestConnectionExit(t *testing.T) {
	var connection server.Connection
	select {
	case <-connection.Exited():
		t.Fatal("exited before Exit")
	default:
	}
	connection.Exit()
	connection.Exit()
	select {
	case <-connection.Exited():
	default:
		t.Fatal("not exited after Exit")
	}
}
