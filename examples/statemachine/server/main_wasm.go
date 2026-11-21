// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

//go:build js && wasm

package main

import (
	"context"
	"errors"
	"os"

	"typefox.dev/fastbelt/examples/statemachine"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
)

func main() {
	ctx := context.Background()

	sc := statemachine.CreateLspServices(func(sc *service.Container) {
		// Override the default services with browser-compatible implementations.
		server.SetupWasmServices(sc)
		sc.Override(workspace.NewNoopInitializer())
	})

	// Send the output of the log and slog packages to the client.
	server.SetDefaultLogger(sc)

	// StartLanguageServer blocks until the client sends the exit notification
	// or the connection is closed, which keeps the WASM instance alive while
	// the worker is running.
	if err := server.StartLanguageServer(ctx, sc); err != nil {
		if errors.Is(err, server.ErrExitWithoutShutdown) {
			// The client disposed the server without shutting it down first:
			// end with exit code 1, as the LSP specification asks, but quietly.
			os.Exit(1)
		}
		// The panic goes to the worker console via the WASM runtime.
		panic(err)
	}
}
