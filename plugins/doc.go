// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package plugins is the home of the plugins maintained as part of Fastbelt.
//
// Each plugin implements [typefox.dev/fastbelt/pluginapi.Plugin] and is
// configured through its struct fields or constructor before it is registered
// in [typefox.dev/fastbelt/cmd.BuildContext.Plugins]. Relative output paths
// in a plugin's configuration are resolved against
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.OutputDir].
//
// Plugins only depend on the public plugin API
// ([typefox.dev/fastbelt/pluginapi] and its subpackages), never on the
// generator internals. See the pluginapi package documentation for how to
// write a plugin.
//
// There are no plugins yet.
package plugins
