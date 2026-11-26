// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package pluginapi defines the interface between the Fastbelt code generator
// and plugins that produce additional output from a grammar, such as syntax
// highlighting definitions or documentation.
//
// # Plugins
//
// A plugin implements [Plugin] and is registered in
// [typefox.dev/fastbelt/cmd.BuildContext.Plugins]. Plugins are configured
// through their constructors or struct fields before they are registered:
//
//	ctx := &cmd.BuildContext{
//		Input:     "./grammar",
//		Languages: []cmd.Language{{Entry: "Model", LanguageID: "mylang"}},
//		Plugins:   []pluginapi.Plugin{&MyPlugin{OutFile: "./syntaxes/mylang.json"}},
//	}
//
// Plugins are available through the Go API only; the fastbelt CLI does not
// support them.
//
// # When plugins run
//
// [typefox.dev/fastbelt/cmd.BuildContext.Build] runs the plugins after all
// generated Go files have been written. The plugins are called sequentially in
// registration order. The first plugin that returns an error aborts the build;
// later plugins are not called, and the error is wrapped with the plugin name.
//
// # Build artifacts
//
// The [BuildArtifacts] are built once per build and shared by all plugins.
// They are read-only by contract: plugins must not modify them, because later
// plugins see the same data.
//
// Relative output paths of a plugin are expected to be resolved against
// [BuildArtifacts.OutputDir].
//
// The artifacts form a pointer graph that may contain cycles (for example,
// child nodes link to their container). They are not meant to be marshalled
// to JSON or similar formats; plugins should pick the data they need.
//
// # Package layering
//
// The plugin API is split into layered packages:
//
//	pluginapi/tokens   token types and token modes (imports nothing)
//	      ▲
//	pluginapi/ast      the grammar AST mirror
//	      ▲
//	pluginapi/types    the type structure derived from the grammar
//	      ▲
//	pluginapi          Plugin, BuildArtifacts (imports all of the above)
//
// Links between artifacts only point down this stack. Reverse lookups (for
// example, from a token type to its declaration in the AST) are provided as
// methods on [BuildArtifacts].
package pluginapi
