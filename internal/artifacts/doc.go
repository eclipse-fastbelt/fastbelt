// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package artifacts converts the internal data of the Fastbelt generator
// (grammar AST, token types, lexer modes) into the public
// [typefox.dev/fastbelt/pluginapi] types handed to plugins.
//
// The conversion lives here so that the plugin API never imports internal
// generator code. The import direction is always internal/artifacts → pluginapi,
// never the reverse.
package artifacts
