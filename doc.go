// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package fastbelt is a language engineering toolkit for Go.
//
// # Overview
//
// Fastbelt covers lexing, parsing, AST creation, cross-reference linking,
// workspace handling, and Language Server Protocol (LSP) support.
//
// Fastbelt is primarily inspired by [Xtext] (Java)
// and [Langium] (TypeScript).
//
// Like these frameworks, Fastbelt uses a grammar definition file as the
// entry point, based on a Grammar language that is itself implemented with
// Fastbelt. See [typefox.dev/fastbelt/grammar] for details.
//
// The [typefox.dev/fastbelt/cmd/fastbelt] command generates Go code from a
// grammar definition, which you can integrate and customize with additional
// code.
//
// # Defining the Grammar
//
// Defining a language in Fastbelt is an iterative design process. You
// co-design the concrete syntax that users write and the abstract syntax tree
// (AST) that your tooling consumes.
//
// In practice, you write interface declarations for AST node types and parser
// rules that describe how instances of those node types are created from input
// text. Each assignment in a parser rule maps matched input to a field on the
// node that the rule builds.
//
// Parser rules always consume contiguous regions of text. They match keywords
// and tokens directly and delegate to other rules for nested regions. This
// keeps grammar structure aligned with language structure and makes it clear
// how parsed text becomes AST subtrees.
//
// Cross-references provide built-in linking from symbol usage to symbol
// definition. Instead of creating a new node, a cross-reference records an
// identifier that the linker resolves to an existing AST node in scope.
//
// Because Go interfaces are generated directly from grammar interfaces, these
// interfaces should reflect how you plan to implement semantic checks,
// interpreters, or code generators. At the same time, there's a close
// correspondence between interfaces and parser rules so the parse model and
// runtime model stay easy to reason about.
//
// The grammar language itself is documented in
// [typefox.dev/fastbelt/grammar], while the code generation workflow is
// documented in [typefox.dev/fastbelt/cmd/fastbelt]. A common workflow is to
// evolve the grammar in small steps and run code generation after each change.
//
// # Customization
//
// There are two ways to add custom behavior to your language:
//
//   - Attach behavior directly to specific generated AST node `Impl` types.
//   - Implement custom services and wire them through the service container.
//
// Validations are implemented with the first approach. To add validation checks
// for a node type, implement [Validator] on that node's generated `Impl`
// struct:
//
//	type PersonImpl struct {
//	    PersonData
//	}
//
//	func (p *PersonImpl) Validate(_ context.Context, _ string, accept fastbelt.ValidationAcceptor) {
//	    if name := p.Name(); name != "" && !unicode.IsUpper([]rune(name)[0]) {
//	        accept(fastbelt.NewDiagnostic(
//	            fastbelt.SeverityError,
//	            "Name must start with an uppercase letter.",
//	            p,
//	            fastbelt.WithToken(p.NameToken()),
//	        ))
//	    }
//	}
//
// This keeps checks close to the node they validate and works naturally with
// generated AST types.
//
// For service-container based customization, define a language-specific
// `SetupServices` function and a `CreateServices` helper. The service container
// API is documented at [typefox.dev/fastbelt/util/service].
//
// By convention, `SetupServices`:
//   - registers custom values and service implementations,
//   - calls needed `SetupDefaultServices` functions from framework packages, and
//   - calls `SetupGeneratedServices` from generated code.
//
// Scaffolded language projects include a generated `services.go` that follows
// this pattern and can be used as a starting point for service-container
// customization.
//
// # Plugins
//
// Code generation can be extended with plugins that produce additional output
// from a grammar, such as syntax highlighting definitions or documentation.
// Plugins are registered in [typefox.dev/fastbelt/cmd.BuildContext] (Go API
// only, not through the CLI) and run after the Go code has been generated.
// They receive a read-only view of the build: languages, token types and
// modes, the grammar source files and AST, and the derived types. See
// [typefox.dev/fastbelt/pluginapi] for writing plugins and
// [typefox.dev/fastbelt/plugins] for the plugins shipped with Fastbelt.
//
// # Syntax Highlighting
//
// The build can generate TextMate grammars for syntax highlighting in editors
// like VS Code. The fastbelt CLI has no option for this; use a small generator
// program with [typefox.dev/fastbelt/cmd.BuildContext] instead, run through
// `go generate`. Scaffolded projects contain one in `gen/main.go`:
//
//	ctx := &cmd.BuildContext{
//	    Languages: []cmd.Language{{
//	        Entry:      "Model",
//	        LanguageID: "mylang",
//	        Patterns:   []string{"**/*.mylang"},
//	    }},
//	    TextMateOutput: "vscode-extension/syntaxes",
//	    TextMateScopes: map[string]string{"SL_COMMENT": "comment.line"},
//	}
//	err := ctx.Build()
//
// TextMateOutput is the directory for the TextMate grammars. If it is empty,
// none are generated. TextMateScopes overrides the scopes that are derived
// from token names and modifiers. The scope rules, the overrides, and how
// token modes are mapped are described in [typefox.dev/fastbelt/grammar].
//
// The build writes two kinds of files:
//
//   - `<grammar>-common.tmLanguage.json` with scope name
//     `source.<grammar>-common`, where `<grammar>` is the lower-cased grammar
//     name. It contains the rules of all languages of the grammar.
//   - `<languageID>.tmLanguage.json` with scope name `source.<languageID>` for
//     each language. It only includes the rules of its language's entry rule
//     from the common file. Its fileTypes are taken from patterns like
//     `**/*.mylang`.
//
// The files are generated, but the extension's `package.json` is not. It
// needs one `grammars` entry per language and one for the common file,
// without a language, so that the language files can include it:
//
//	"contributes": {
//	  "grammars": [
//	    {
//	      "language": "mylang",
//	      "scopeName": "source.mylang",
//	      "path": "./syntaxes/mylang.tmLanguage.json"
//	    },
//	    {
//	      "scopeName": "source.mylangmodel-common",
//	      "path": "./syntaxes/mylangmodel-common.tmLanguage.json"
//	    }
//	  ]
//	}
//
// Here the grammar is named `MyLangModel`. The language ID must match the
// `languages` entry of `package.json`.
//
// # To Go Further
//
// For implementing the exact behavior of your language, these subpackages are
// the most relevant:
//
//   - [typefox.dev/fastbelt/linking] for everything related to
//     cross-reference linking, including scopes and making symbols reachable
//     between files.
//   - [typefox.dev/fastbelt/workspace] for handling a collection of files in a
//     workspace folder, processing file changes by executing phases such as
//     parsing, linking, and validating.
//   - [typefox.dev/fastbelt/server] for everything related to the Language
//     Server Protocol (LSP).
//
// [Xtext]: https://eclipse.dev/Xtext/
// [Langium]: https://langium.org
package fastbelt
