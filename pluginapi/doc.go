// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package pluginapi defines the interface between the Fastbelt code generator
// and plugins that produce additional output from a grammar, such as syntax
// highlighting definitions or documentation.
//
// # Writing a plugin
//
// A plugin implements [Plugin]: Name identifies it in error messages, and
// Build receives the [BuildArtifacts] of the build and writes whatever the
// plugin produces. Plugin options are struct fields or constructor
// parameters; there is no further configuration mechanism.
//
//	type MyPlugin struct {
//		// OutFile is resolved against BuildArtifacts.OutputDir if relative.
//		OutFile string
//	}
//
//	func (p *MyPlugin) Name() string { return "my-plugin" }
//
//	func (p *MyPlugin) Build(artifacts pluginapi.BuildArtifacts) error {
//		out := p.OutFile
//		if !filepath.IsAbs(out) {
//			out = filepath.Join(artifacts.OutputDir, out)
//		}
//		var b strings.Builder
//		for _, tt := range artifacts.TokenTypes {
//			if tt.Kind == tokens.KindKeyword {
//				fmt.Fprintln(&b, tt.Keyword)
//			}
//		}
//		return os.WriteFile(out, []byte(b.String()), 0o644)
//	}
//
// See the package example for a complete, runnable plugin. Plugins
// maintained as part of Fastbelt live in [typefox.dev/fastbelt/plugins].
//
// # Registering a plugin
//
// Plugins are registered in [typefox.dev/fastbelt/cmd.BuildContext.Plugins],
// configured before they are added:
//
//	ctx := &cmd.BuildContext{
//		Input:   "./grammar",
//		Output:  "./generated",
//		Package: "farewell",
//		Languages: []cmd.Language{
//			{Entry: "Farewell", LanguageID: "farewell", Patterns: []string{"**/*.bye"}},
//		},
//		Plugins: []pluginapi.Plugin{
//			&MyPlugin{OutFile: "./keywords.txt"},
//		},
//	}
//	if err := ctx.Build(); err != nil {
//		log.Fatal(err)
//	}
//
// Plugins are available through the Go API only; the fastbelt CLI
// (cmd/fastbelt) does not support them. Call the build from a small Go
// program, for example one run by go generate.
//
// # Execution order and errors
//
// [typefox.dev/fastbelt/cmd.BuildContext.Build] runs the plugins after all
// generated Go files have been written, so a plugin may read them. The
// plugins are called sequentially in registration order, never concurrently.
//
// The build fails fast: the first plugin that returns an error aborts the
// build, later plugins are not called, and Build returns the error wrapped
// with the plugin name (plugin "my-plugin": ...); use [errors.Is] or
// [errors.As] to inspect the original error. A panic in a plugin is handled
// the same way: it is recovered and returned as an error named after the
// plugin (plugin "my-plugin": panicked: ...) that includes the stack trace;
// if the panic value is an error, it is wrapped. Files written before the failure,
// including the generated Go files, are not removed. A nil plugin is rejected
// before anything is generated.
//
// # Paths
//
// [BuildArtifacts.InputDir] and [BuildArtifacts.OutputDir] are absolute.
// The build does not resolve paths configured on a plugin; each plugin
// resolves its relative output paths against OutputDir itself (as MyPlugin
// above does), not against the current working directory, so that a build
// behaves the same wherever it is started. [BuildArtifacts.Package] is the
// Go package name of the generated code.
//
// # Artifacts
//
// [BuildArtifacts] holds the following artifacts:
//
//   - [BuildArtifacts.Languages]: the configured languages, see [Language].
//     Each one links to the token mode its lexer starts in
//     ([Language.EntryMode]) and to its entry parser rule
//     ([Language.EntryRule]).
//   - [BuildArtifacts.TokenTypes]: one [tokens.TokenType] per runtime token
//     index, with EOF at index 0. A token type knows its kind (keyword,
//     regexp, group), its keyword value or regexp source, its aliases and,
//     for groups, its member token types.
//   - [BuildArtifacts.TokenModes]: the effective [tokens.TokenMode]s of the
//     generated lexer, each listing its members in lexer priority order with
//     the modifier and command (push, pop, mode) effective in that mode. A
//     command links to its target mode.
//   - [BuildArtifacts.Sources]: the .fb files of the build as
//     [ast.SourceFile]s, sorted by path.
//   - [BuildArtifacts.Grammar]: the [ast.Grammar], a typed mirror of the
//     merged grammar after infix expansion. Nodes link to their container,
//     to the target of cross-references, to their [ast.Location] in a
//     source file and, for token-producing or token-consuming nodes, to their
//     [tokens.TokenType].
//   - [BuildArtifacts.Types]: one [types.Type] per grammar interface, with
//     resolved supertypes, subtypes and fields, the parser rules returning
//     the type, and a link back to the [ast.Interface] declaring it.
//
// Indices are identities: TokenTypes[i].Index == i and TokenModes[i].Index
// == i, and both match the IDs the generated lexer uses at runtime. Every
// pointer to a token type, token mode, AST node or type points into these
// slices or into Grammar; the same element is always the same pointer, so
// pointers can be compared and used as map keys.
//
// Links only point down the package layering described below. Lookups in
// the other direction are methods on [BuildArtifacts], such as
// [BuildArtifacts.TokenDeclaration], which maps a token type to its
// declaration in the AST.
//
// # Locations and line/column positions
//
// An [ast.Location] is a byte range: Start and End are byte offsets into the
// Text of its [ast.SourceFile], as produced by the parser, so
// Source.Text[Start:End] (or [ast.Location.Text]) is the located text. To
// report a position to users, convert an offset with
// [ast.SourceFile.LineColumn], which returns a 1-based line and a 1-based
// column counted in runes, not bytes:
//
//	loc := rule.Location()
//	line, col := loc.Source.LineColumn(loc.Start)
//	fmt.Printf("%s:%d:%d: %s\n", loc.Source.Path, line, col, rule.Name)
//
// The source tokens of the grammar files are not part of the API: there is
// no token stream and no per-token node. Node values are plain Go values
// (names, unquoted keywords, regexps without slashes), and node locations
// cover the whole node.
//
// # Read-only, shared artifacts
//
// The artifacts are built once per build, only when at least one plugin is
// registered, and the same values are passed to every plugin. They are
// read-only by contract: plugins must not modify them (including slices and
// maps reachable from them), because later plugins see the same data. A
// plugin that needs a different view should copy the parts it changes.
//
// # Pointer graph
//
// The artifacts form a pointer graph that contains cycles (for example, a
// child node links to its container, a type to its subtypes and their
// supertypes). They are not meant to be marshalled to JSON or similar
// formats, and naive recursive traversal does not terminate; plugins should
// pick the data they need and walk owned children only (such as the fields
// of a node rather than its Container).
//
// # Package layering
//
// The plugin API is split into layered packages:
//
//	pluginapi/tokens   token types and token modes (imports nothing)
//	      ▲
//	pluginapi/ast      the grammar AST mirror, source files and locations
//	      ▲
//	pluginapi/types    the type structure derived from the grammar
//	      ▲
//	pluginapi          Plugin, BuildArtifacts, Language (imports all of the above)
//
// Each package only imports the packages below it, and links between
// artifacts only point down this stack.
//
// The plugin API does not depend on the generator: the conversion from the
// generator's internal data lives in internal/artifacts and is called by
// [typefox.dev/fastbelt/cmd]. The only exception is the dependency-free hook
// package internal/artifacts/astinit, which pluginapi/ast and pluginapi
// import to let the conversion set unexported state (the container and
// location of AST nodes, the map behind [BuildArtifacts.TokenDeclaration]).
// Being internal, it cannot be imported outside the Fastbelt module, so
// plugins cannot change that state; plugins inside the module (see
// [typefox.dev/fastbelt/plugins]) must not import it either.
package pluginapi
