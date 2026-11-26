// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package ast is a read-only mirror of the Fastbelt grammar AST, as handed to
// plugins via [typefox.dev/fastbelt/pluginapi.BuildArtifacts].
//
// Nodes may link down to [typefox.dev/fastbelt/pluginapi/tokens], but never up
// to [typefox.dev/fastbelt/pluginapi/types] or [typefox.dev/fastbelt/pluginapi].
package ast
