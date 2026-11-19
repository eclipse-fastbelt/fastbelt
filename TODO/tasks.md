# Tasks: TextMate generation

The design is in [README.md](README.md). Do the tasks roughly in this order.

## 0. Rebase ✅ done (2026-11-17)

- [x] Rebase `lotes/textmate` onto `origin/main` (`c8db26b`). The CLI conflicts were resolved in favour of main,
  which dropped the draft's `--textMateOut` wiring, as the plan says.
- [x] Adapt the draft to the current grammar AST:
  - `TokenDecl.Content` (regex or keyword content)
  - token modifiers
  - `grammar.KeywordValue` / `grammar.RegexpValue`
- [x] `go build ./...`, `go test -race ./...`, golangci-lint v2.14.0 and the VS Code extension build (`npm run build`) pass.

## 1. Regex handling (`internal/generator/regexp_utils.go`) ✅ done (2026-11-17)

- [x] Rewrite begin/end extraction on `regexp/syntax`: `SplitDelimiters(pattern) ([]Delimiters, error)` replaces the
  VM-based `GetTerminalParts`. It expands alternatives in source order, removes duplicates, and returns
  `ErrTooManyAlternatives` above 32 alternatives.
  - [x] `OpQuest`: optional parts now count as variable, so no branch is lost.
  - [x] No panic on `^`, `$`, `\A`, `\z`, `\b`, `\B`. These are zero-width parts of the begin/end.
    Assertions alone never form a delimiter.
  - [x] `x+` keeps its required leading `x` (`a+b` → begin `a`, end `b`).
  - [x] Parse errors are returned.
- [x] Re-emit Oniguruma regex from the `syntax.Regexp` tree with `ToOniguruma(pattern) (string, error)`:
  - [x] `(?P<name>…)` → `(?<name>…)`
  - [x] Flags are resolved by the Go parser:
    - `(?s).` → `[\s\S]`
    - `(?m)` → `^`/`$`; without it → `\A`/`\z`
    - `(?U)` flips greediness
    - `(?i)` literal → `(?i:…)`
  - [x] Character class escaping, including `&` because `&&` is class intersection in Oniguruma.
    Negated classes are emitted as `[^…]` when that is shorter. Control characters are written as `\x{…}`.
- [x] Unit tests for each case above (`regexp_utils_test.go`).
- [x] One-off check against real `vscode-oniguruma`: for 36 pattern/input pairs, the converted pattern matches the
  same text as Go. Not committed; it would need npm in the Go tests.
- Note: the TextMate generator now uses `SplitDelimiters`, but its `match` patterns (keywords) still use
  `regexp.QuoteMeta`. Task 3/4 switches them to `ToOniguruma`.

## 2. Mode model for TextMate ✅ done (2026-11-17)

- [x] Reuse `GenerateTokenTypes` / `lexerModes()` to get each mode's keywords, tokens, modifiers and commands,
  with keyword selectors resolved. The new `buildTextMateModel` in `internal/generator/textmate_modes.go`
  produces `textMateModel{Modes, ModesByName, Languages}`.
  - Each `textMateToken` has: `Name`, `Keyword` (unquoted) or `Regexp` (no slashes), `Modifier` for this mode,
    and `Command` + `Target`.
  - Commands with an unknown target mode are dropped, like the lexer does.
  - The lexer's per-mode filter (reachability) moved into `lexerMode.tokenIndices`, which both generators share.
    After regenerating all 10 languages, no generated file changed.
- [x] Find each language's entry mode: the declared `Language.TokenMode`, `default`, or the generated per-entry mode.
  - The per-entry mode's **name** is the entry rule name (`Greeting`). Its lexer constant is `TokenMode_Greeting`.
  - So README's repository keys become `Greeting` (language) and `mode_Greeting` (mode).
- [x] `textMateMode.PopTokens()` returns each mode's pop tokens, used as the `end` of the rules that push into that mode.
- [x] Tests: `textmate_modes_test.go` has 8 tests:
  - implicit default
  - no entries
  - keyword token
  - push/pop/mode with modifiers that differ per mode
  - unknown target
  - keyword selector
  - generated and declared multi-language modes
- Note: keyword `Name`s come from `grammar.KeywordName`. Letters keep their case (`hello`), punctuation is named
  (`Semicolon`, `RightBrace`). Task 4 uses them for repository keys of tokens with commands.

## 3. Scopes ✅ done (2026-11-17)

- [x] Heuristic scope function `textMateScopes.scopeOf(token, Delimiters)` in `internal/generator/textmate_scopes.go`.
  It returns a scope with the `.<grammar>` suffix, or `""`. The rules, in order:
  - An override for the token name. (Task 7: overrides now also apply to `hidden` tokens.)
  - `hidden` gets no scope.
  - `comment` → `comment.block` if the split has an end, else `comment.line`.
  - Keywords with letters → `keyword.control`; `;` → `punctuation.terminator`; only `,.:` → `punctuation.separator`;
    only `(){}[]` → `punctuation.section`; other symbols → `keyword.operator`.
  - Name words (split at `_`, digits and camelCase, so `POINT` does not match `INT`):
    `STRING`/`STR` → `string.quoted.{double,single,backtick,other}`, chosen by the begin delimiter;
    `INT`/`INTEGER`/`NUMBER`/`NUM`/`FLOAT`/`DOUBLE`/`DECIMAL`/`REAL`/`HEX` → `constant.numeric`.
  - Everything else (e.g. `ID`) → no scope.
- [x] `keywordsPattern([]string) string` replaces the draft's `GroupKeywords`, which wrapped the quoted text in `\B…\B`
  and broke `a+`. A `\b` is added only on sides that start or end with a word character (Unicode-aware, like Oniguruma).
  Keywords are sorted longest first. Keywords with the same boundaries share a group (`\b(?:while|class|if)\b`),
  unless that would put a keyword before a longer keyword it is a prefix of (`\bif\(|\bif\b`).
  The draft's `CaseInsensitive` option was removed; the grammar has no case-insensitive keywords.
- [x] Overrides: `textMateScopes.Overrides` (token name → scope without the suffix; `""` = no scope). Keywords without a
  token declaration use their `grammar.KeywordName` (`Semicolon`). Task 6 fills it from `BuildContext.TextMateScopes`.
- [x] Tests: `textmate_scopes_test.go`.

## 4. Common file generator ✅ done (2026-11-17)

`GenerateTextMateCommon(grammar, entryRules, entryModes, tokenTypes, overrides) (TextMateCommonResult{Content, Warnings}, error)`
in `internal/generator/textmate_generator.go` replaces the draft. `TextMateGrammarName(grammar)` gives the lower-cased name.

- [x] Repository entries:
  - `<EntryRule>` → include `#mode_<EntryMode>`. Nothing is emitted when there are no entry rules.
  - `mode_<Mode>`: includes only
  - `keywords_<Mode>`: one `match`. If the keywords have different scopes, each group is a capture with its own scope,
    so that one regex keeps the longest-first order across scopes (`:=` before `:`).
  - `token_<Name>`, `token_<Name>_comment`, `token_<Name>_hidden`
  - Command rules: `token_<Name>[_<mod>]_push_<T>` and `token_<Name>[_<mod>]_mode_<T>_in_<Mode>`.
    The `mode` rule's end depends on the parent mode, so the parent is part of the key.
- [x] `hidden` tokens get no scope unless overridden (task 7). They are emitted as plain `match` to consume text,
  except tokens that can only match whitespace, which are left out.
- [x] Comment and string tokens (by modifier or scope) are split with `SplitDelimiters`. They become `begin`/`end`
  (one rule per alternative) when at least one alternative has an end delimiter, and alternatives without an end
  get `end: "$"`. Otherwise (line comments, no literal begin, too many alternatives) they stay one `match`.
  Strings whose regex contains a literal `\` get an inner `\\.` → `constant.character.escape` pattern.
  - Limitation: the escape pattern is added to every alternative of that token (`'[^']*'` too).
    Go's parser factors out common prefixes, so a body cannot reliably be mapped back to its split part.
- [x] Token-mode commands:
  - [x] `push(M)`: `begin` = the token (scope in `beginCaptures`), `end` = M's pop tokens. With several pop tokens,
    each is wrapped in a capture for `endCaptures`, and captures inside a token regex shift the numbering.
    `patterns` = include `#mode_M`.
  - [x] Mode with no pop: `end: "(?!)"`
  - [x] `mode(M)` in P: `end: "(?=<P's pops>)"`, or `(?!)` if P has none. A warning is returned when M's pop tokens
    differ from P's. Printing it is left to the caller (task 6).
  - [x] Pop tokens are not emitted inside their own mode's patterns.
    - Limitation: pop tokens in an entry mode are therefore not highlighted at the root.
- [x] Pattern order inside a mode: command rules, comment/string tokens, keywords, other tokens (documented on
  `GenerateTextMateCommon`).
- [x] Deterministic JSON: `encoding/json` sorts the map keys, and `SetEscapeHTML(false)` keeps `&&`/`<` readable.
- [x] Tests: `textmate_generator_test.go`. Also a one-off check with real `vscode-textmate` + `vscode-oniguruma`
  on the token_modes grammar and a basic grammar (not committed). Nested interpolation, multi-line comments,
  escapes, `//` vs `/`, and `iffy` vs `if` all tokenize as expected.

## 5. Language file generator ✅ done (2026-11-17)

- [x] `GenerateTextMateLanguage(grammar, entry, Selector) (string, error)` writes `<languageID>.tmLanguage.json`:
  - `name: <languageID>`, `scopeName: source.<languageID>`
  - `patterns: [{include: "source.<grammar>-common#<EntryRule>"}]`
  - `fileTypes` comes from the selector patterns `*.ext`, `**/*.ext` and `*.{a,b}`. It is written without the dot,
    as is the TextMate convention (the draft wrote `.fb`). Other globs are skipped, and the field is left out
    when none match.
  - An empty entry or language ID is an error.
- [x] File name helpers for task 6: `TextMateCommonFileName(grammar)`, `TextMateLanguageFileName(languageID)`.
- [x] Tests in `textmate_generator_test.go`.

## 6. Wiring ✅ done (2026-11-17)

- [x] `cmd.BuildContext`: `TextMateOutput string` (a directory, relative to the working directory like `Input`/`Output`;
  empty = no TextMate files) and `TextMateScopes map[string]string`, both documented on the fields.
- [x] `cmd.Generate` now takes `GenerateOptions{ATN, Railroad, Verbose, TextMateOutput, TextMateScopes}` instead of
  three positional bools. This changes the signature of an exported function; the only other caller is the CLI.
  It writes `<grammar>-common.tmLanguage.json` and one `<languageID>.tmLanguage.json` per language, and prints the
  generator's warnings as `Warning: …`. Without one selector per entry it returns an error.
- [x] Language ID, entry rule and entry mode come from `BuildContext.Languages` (`entries`, `entryModes`, `selectors`).
  - When `TextMateOutput` is set, `Build` rejects language IDs containing `/` or `\`, and an ID whose file name would
    overwrite the common file (`<grammar>-common`).
  - Overrides for token names that no mode uses produce a warning.
- [x] `fastbelt generate` CLI: the draft's `--textMateOut` flag had already gone in the rebase; only the `Generate`
  call changed. No new flags.
- [x] The draft's `grammar.CreateServices()` lookup had already gone in the rebase as well.
- [x] Tests: three `TestBuild…TextMate…` tests in `cmd/build_test.go` and
  `TestTextMateCommonWarnsAboutUnknownOverrides`. `go generate ./...` changes no files.

## 7. Tests ✅ done (2026-11-17)

`cmd/textmate_golden_test.go` (`TestTextMateGolden`) runs `BuildContext.Build` with `TextMateOutput` for each case
and compares the produced file set and contents with `cmd/testdata/textmate/<case>/`.
`go test ./cmd -run TestTextMateGolden -update` rewrites them.

- [x] `multilang`: several files, two languages, declared per-language modes (config copied from its `gen/main.go`).
- [x] `token_modes`: push, pop, nested modes.
- [x] `arithmetics` and `statemachine`: language IDs and extensions from their VS Code `package.json`.
  arithmetics uses `SL_COMMENT → comment.line`, which its `gen/main.go` will need in task 8.
  - For task 8: the goldens currently live in testdata, because the extensions' `syntaxes/` are still hand-written.
    Once task 8 generates them, either point these two cases at `internal/vscode-extensions/<ext>/syntaxes`
    or keep testdata and rely on `go generate` producing no diff.
- [x] `strings`: a new test grammar `cmd/testdata/textmate/input/strings/strings.fb` with:
  - a double-quoted string with escapes
  - a multi-line backtick string
  - a char token scoped through an override
  - a comment with a line and a block alternative
  - a hidden non-whitespace token
  - `let!`/`let` and `:=`/`:` ordering
- [x] Bug found while reviewing the goldens and fixed: arithmetics' `hidden token SL_COMMENT` was sorted after the
  keywords, so `// x` was highlighted as two `/` operators, and it could not get a scope at all. Now:
  - overrides apply to hidden tokens (no heuristic scope)
  - hidden tokens, and tokens whose scope is `comment.*`/`string.*`, are sorted before keywords together with
    comments and strings, and may become begin/end rules
  - hidden whitespace is still left out, unless an override gives it a scope
  - tests: `TestTextMateCommonHiddenCommentBeforeKeywords` and the updated override tests
- [x] One-off check of the arithmetics and strings goldens with real `vscode-textmate` (not committed). Every token
  in the sample lines got the expected scope.

## 8. VS Code extensions (`internal/vscode-extensions`) ✅ done (2026-11-17)

- [x] arithmetics and statemachine:
  - [x] `examples/<x>/gen/main.go` with `cmd.BuildContext`; `go:generate go run ./gen`.
    - Same language ID and `**/*.calc` / `**/*.statemachine` patterns as the hand-written selectors.
    - `TextMateOutput: ../../internal/vscode-extensions/<x>/syntaxes`.
    - arithmetics: `TextMateScopes{"SL_COMMENT": "comment.line"}`. statemachine: `ATN: true`.
  - [x] Generated `syntaxes/arithmetics-common.tmLanguage.json` + `arithmetics.tmLanguage.json`, and
    `statemachinemodel-common.tmLanguage.json` + `statemachine.tmLanguage.json`. The common file is named after the
    grammar (`StatemachineModel`), not the language.
  - [x] Removed the hand-written `sc.Put[core.LanguageSelector]` from both `services.go`. The only Go change from
    regenerating is that `services_gen.go` now registers the same selector.
  - [x] The hand-written `<id>.tmLanguage.json` files are overwritten by the generated ones (same names).
  - [x] `package.json`: added a `grammars` entry for each common file (`scopeName` + `path`, no `language`).
  - [x] `TestTextMateGolden` compares the arithmetics and statemachine cases with the extensions' `syntaxes/`
    (same config as the `gen/main.go`), so it also fails when they are out of date. Their testdata copies were removed.
    `-update` only replaces `*.tmLanguage.json` in those directories.
- [x] `fastbelt` (the grammar language): keep the hand-written `data/fastbelt.tmLanguage.json`. It highlights the
  inside of regex literals (classes, escapes, groups), which a one-scope-per-token generator cannot do.
  - Possible follow-up: generate `fastbelt-common` and keep only a small hand-written file that puts the regex
    rules first and includes `source.fastbelt-common#Grammar`, so keywords follow `grammar.fb`.
- [x] Lint workaround: staticcheck v0.8.1 (golangci-lint v2.14.0, also used by CI) panicked in SA4023
  (`nilness.go:77`, index out of range) on `examples/arithmetics/cli` once the generated selector code was in the
  package. It is triggered by `x, err := sc.Get[T](); if err != nil`, apparently a mix-up of facts between the Go 1.27
  generic methods `Has`/`Get`. The CLI now uses `sc.MustGet`, with a comment. Worth reporting upstream.
- [x] Checks:
  - `go test -race ./...`, golangci-lint and `npm run build` pass.
  - One-off `vscode-textmate` run (not committed) over `example.calc` and `example.statemachine`, with the grammars
    loaded through `package.json`: keywords, punctuation, operators, numbers and `//` comments as expected.

## 9. Scaffold (`internal/scaffold`) ✅ done (2026-11-19)

- [x] New templates `gen.go.tmpl` (only `//go:generate go run ./gen`, like the examples) and `gen/main.go.tmpl`, a
  `cmd.BuildContext` with `Input: "."`, `Package: <GoPackage>` (it can differ from the directory name), entry rule,
  language ID and `**/*.<ext>`. `TextMateOutput: "vscode-extension/syntaxes"` is only set with `--vscode`.
- [x] `services.go.tmpl`: removed the `go:generate` line, the hand-written `LanguageSelector` and the root
  `typefox.dev/fastbelt` import. `SetupGeneratedServices` registers the selector.
- [x] The `go get -tool …/cmd/fastbelt` step was dropped. The scaffold now runs `go get typefox.dev/fastbelt/cmd[@v]`.
  Getting that package, not only the module, records the go.sum entries that `go run ./gen` needs before `go mod tidy`.
- [x] `package.json.tmpl`: second `grammars` entry for the common file (no `language`).
  - `SyntaxFile`/`ScopeName` now use the language ID (`my-lang.tmLanguage.json`, `source.my-lang`), as the generator
    does. Before, they used the extension without dashes. New `CommonSyntaxFile`/`CommonScopeName`
    (`mylangmodel-common`).
- [x] Dropped `syntaxes/language.tmLanguage.json.tmpl`. `Run` always runs `go generate`, which writes both grammars.
- [x] `README.md.tmpl`, the root `README.md` and the fastbelt extension's README no longer mention the tool dependency.
  They describe `gen/main.go` and the generated grammars.
- [x] Tests:
  - `scaffold_test.go`: names, `gen/main.go` with and without `--vscode`, `gen.go`, `services.go`, and the
    `package.json` grammars
  - e2e: the generated grammars exist and their `scopeName`s match `package.json`
  - Manual: scaffold without `--vscode` and into an existing module (`-p langs/calc`). Both build.
- Found, not fixed (already there before): `scaffold -m example.com/acme/host -p pkg/lang` runs `go mod init` with
  the package import path (`…/host/pkg/lang`), so `cmd/<lsp>/main.go` does not build.

## 10. Docs

- [ ] Document TextMate generation: the `BuildContext` fields, scope heuristics and overrides, mode mapping and its limits,
  and the `package.json` snippet. Put it in `doc.go` and/or `grammar/doc.go`.
- [ ] Side note: `workspace/doc.go` still refers to `[FileExtensions]`, which no longer exists. Fix it while you are there.
