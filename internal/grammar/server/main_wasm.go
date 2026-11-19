// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

//go:build js && wasm

package main

import (
	"context"

	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
)

func main() {
	ctx := context.Background()

	sc := CreateServices(func(sc *service.Container) {
		server.SetupWasmServices(sc)
		// The WASM server runs in a worker without file access of its own:
		// workspace files are requested from the language client instead.
		sc.Override[workspace.FileSystem](grammar.NewClientFileSystem(sc))
	})

	// StartLanguageServer blocks on the connection until it is closed, which
	// keeps the WASM instance alive while the worker is running.
	if err := server.StartLanguageServer(ctx, sc); err != nil {
		// Logging goes to the worker console via the WASM runtime.
		panic(err)
	}
}
