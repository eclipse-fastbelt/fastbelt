// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

//go:build !js || !wasm

package main

import (
	"context"
	"fmt"
	"os"

	"typefox.dev/fastbelt/examples/statemachine"
	"typefox.dev/fastbelt/server"
)

func main() {
	ctx := context.Background()
	sc := statemachine.CreateLspServices(server.SetupStdioServices)

	// Send the output of the log and slog packages to the client.
	server.SetDefaultLogger(sc)
	if err := server.StartLanguageServer(ctx, sc); err != nil {
		// The client is gone by now, and stdout may be closed: report the error
		// on stderr instead of through the logger, which writes to the client.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
