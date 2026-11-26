# Prompt

I would like to create a plugin system for this project.
Please create a /pluginapi folder with an pluginapi.go inside of it. This file shall define an interface with this definition: { Name() string, Build(artifacts pluginapi.BuildArtifacts) error }
The type pluginapi.BuildArtifacts needs to be defined. In summary pluginapi.BuildArtifacts shall contains all artifacts that were built in the Fastbelt generation processes. These processes and their types are internal, so for BuildArtifacts we need a kind of copy for these types and copy them over from the internal processes.
Plugins are called inside of a cmd.BuildContext. This type shall get a list of plugins whose constructors contain the configuration of these plugins (Example 1).

There should be also a folder /plugins where all homegrown Fastbelt plugins shall be located.

Back to the build artifacts. They shall contain:

* the languages that were given to the build context
* the tokens and the token modes
  * connect them via token indices
* the AST
  * avoiding pointers to Reference[T], prefer *T
  * avoiding also pointers to tokens, use a token index instead
* the interfaces and types structure derived from the AST, so they are descriptive types not Go interfaces/types

When you plan, organize AST and types at the end of the plan, languages and tokens+modes have precedence.
Build one artifact at a time, make it complete before moving to the next one. Add tests for each artifact to ensure correctness.

Example 1:

```go
ctx := &BuildContext{Input: ..., Output: ..., Package: "multi", Languages: []Language{
    {Entry: "Farewell", TokenMode: "Nope", LanguageID: "farewell"},
}, Plugins: []pluginapi.Plugin{
    &plugins.TextMatePlugin{
        CommonOutDir: "./syntaxes/common.tmLanguage.json",
        LanguageOutDirs: map[string]string{
            "farewell": "./syntaxes/farewell.tmLanguage.json"
        }
    },
}}
```

# Summary

Result of the design discussion. These decisions are binding for all steps in `tasks/`.

## Plugin API

- `pluginapi.Plugin { Name() string; Build(artifacts pluginapi.BuildArtifacts) error }` – artifacts passed **by value**, no `context.Context`.
- `cmd.BuildContext.Plugins []pluginapi.Plugin`; plugins are configured via their constructors/struct fields (Example 1).
- Plugins run **after** all `*_gen.go` files are written, sequentially in registration order, **fail fast**; errors are wrapped with the plugin name.
- Artifacts are built **once** and shared; they are read-only by contract.
- Relative output paths of plugins resolve against `BuildArtifacts.OutputDir`; `InputDir` and `Package` are provided too.
- Plugins are available through the **Go API only** (no `cmd/fastbelt` CLI support).
- `/plugins` holds homegrown plugins; it starts empty. The TextMate plugin of Example 1 is only an illustration and a separate follow-up.

## Packages and layering

```
pluginapi/tokens   TokenType, TokenMode, TokenModeMember, TokenCommand,  (imports nothing)
                   Modifier, CommandKind
      ▲
pluginapi/ast      grammar AST mirror + SourceFile, Location             (imports tokens, internal/artifacts/astinit)
      ▲
pluginapi/types    Type, Field, FieldType                                (imports ast)
      ▲
pluginapi          Plugin, BuildArtifacts, Language                      (imports all)
```

- Links (pointers) only point **down** this stack. Reverse lookups are methods on `BuildArtifacts` (e.g. `TokenDeclaration(tt)`).
- `pluginapi` never imports internal generator code; the conversion lives in `internal/artifacts`, called from `cmd`.
  The **only** exception is `internal/artifacts/astinit`: a dependency-free hook package (only `func` variables with
  `any` parameters, imports nothing – in particular not `ast`, to avoid a cycle). `pluginapi/ast` fills the variables
  in its `init()`; the converter calls them to set unexported node state (container, location). Go's `internal` rule
  allows this import (both live under the module root), while plugin authors cannot import `astinit` themselves.
- Pointers between artifacts are fine as long as they point to `pluginapi*` types. Indices are kept as identity (`TokenTypes[i].Index == i`, `TokenModes[i].Index == i`) and match the runtime IDs. This replaces the original wording "connect them via token indices".
- Artifacts form a pointer graph (incl. cycles); they are **not** meant to be JSON-marshalled.

## Artifacts (in implementation order)

1. **Languages** – `pluginapi.Language{LanguageID, Entry, Patterns, EntryMode *tokens.TokenMode, EntryRule *ast.ParserRule}`.
2. **Token types** – one `tokens.TokenType` per runtime index (dense, EOF = 0): name, aliases (shared indices such as `"a"` / `token A: "a"`), kind, keyword value / regexp source, group members, declared modifier, synthetic flag. No automata.
3. **Token modes** – the **effective** modes of the generated lexer (incl. synthesized per-language modes; implicit `default` dropped when modes are declared), members in lexer priority order (keywords first) with per-mode modifier and command (`push`/`pop`/`mode` + target mode).
4. **Sources and locations** – `ast.SourceFile{Path, Text}` (created via `ast.NewSourceFile`, which precomputes an unexported line index), `BuildArtifacts.Sources` sorted by path, `(*SourceFile).LineColumn(offset) (line, col)` (1-based line, 1-based **rune** column, `\n`/`\r\n`/`\r` line breaks, offset clamped), `ast.Location{Source *SourceFile; Start, End int}` with **byte** offsets (as produced by the parser, `core.TextRange`) and `Location.Text()` (`""` if `Source == nil`).
   **Source tokens are not part of the API** (decided against): no `ast.Token`, no token stream, no `*core.Token` mapping. This supersedes the original wording "use a token index instead".
5. **AST** – typed mirror of the **merged grammar after infix expansion**, exact grammar names, sealed Go interfaces for unions, `Reference[T]` → `*T`, `*core.Token` fields → plain values (`string` for names/values/regexps – keywords unquoted, regexps without slashes; `bool` for `?=` flags; typed string constants for fixed choices such as `Cardinality`, operators, `Associativity`, `PrimitiveType.Type`, and the shared `tokens.Modifier` / `tokens.CommandKind`), `Container()` parent links, `Location()` (whole-node byte range; synthetic infix nodes inherit the range of the infix rule/operator they came from; the merged multi-file `Grammar` root has a zero `Location`), `Synthetic` flag, unresolved references are a conversion error. Token-consuming nodes (`Keyword`, `RuleCall` to a token/group) and `TokenDecl`/`TokenGroup` link to `*tokens.TokenType`. No reverse "usages" list. A completeness test against `internal/grammar/grammar.fb` keeps the mirror in sync.
   No `Doc` field / doc comments on AST nodes (decided against).
6. **Types** – descriptive, resolved view of the grammar interfaces: supertypes (direct/transitive), subtypes, own/all fields, field type descriptors (Primitive/Node/Reference/Array), `ReturnedBy` parser rules, link back to the AST declaration.

## Process

- One artifact at a time, complete with tests before moving on; steps in `tasks/01-…` to `tasks/09-…`.
- Tests use explicit assertions, **no golden files**; fixtures are inline grammars plus `examples/arithmetics`, `internal/languages/{multilang,token_modes,token_groups}`.
