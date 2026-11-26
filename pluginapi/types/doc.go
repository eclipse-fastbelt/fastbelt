// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package types describes the resolved type structure (interfaces, fields,
// supertypes) derived from a Fastbelt grammar, as handed to plugins via
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts].
//
// Types may link down to [typefox.dev/fastbelt/pluginapi/ast], but never up to
// [typefox.dev/fastbelt/pluginapi].
package types
