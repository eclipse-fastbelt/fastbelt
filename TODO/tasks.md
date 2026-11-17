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
  - `hidden` never gets a scope, not even with an override.
  - An override for the token name.
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
- [x] `hidden` tokens never get a scope. They are emitted as plain `match` to consume text,
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

## 5. Language file generator

- [ ] `<languageID>.tmLanguage.json`: `name`, `scopeName: source.<languageID>`,
  `patterns: [{include: "source.<grammar>-common#<EntryRule>"}]`.
  `fileTypes` comes from the language patterns where they are simple extensions; VS Code ignores it anyway.

## 6. Wiring

- [ ] `cmd.BuildContext`: add `TextMateOutput string` and `TextMateScopes map[string]string`, and pass them to `cmd.Generate`.
- [ ] Language ID, entry rule and entry mode for each TextMate file come from `BuildContext.Languages`.
- [ ] `fastbelt generate` CLI: remove the draft's `--textMateOut` flag (`cmd/fastbelt/main.go`, `generate.go`). Add no new flags.
- [ ] Remove the draft's use of `grammar.CreateServices()` for the ID and extensions.

## 7. Tests

- [ ] Golden test on `internal/languages/multilang`: several files, two languages, per-language modes.
- [ ] Golden test on `internal/languages/token_modes`: push, pop, nested modes.
- [ ] Golden tests for arithmetics and statemachine through the generated extension files.
- [ ] A test with a string token to cover the string patterns and escapes.

## 8. VS Code extensions (`internal/vscode-extensions`)

- [ ] arithmetics and statemachine:
  - [ ] Change `examples/arithmetics` and `examples/statemachine` from the CLI `go:generate` line to a
    `gen/main.go` with `cmd.BuildContext` (one `Language` each, `TextMateOutput` pointing at the extension's `syntaxes/`).
    Keep the existing `--atn` option for statemachine (`BuildContext.ATN`).
  - [ ] Generate `syntaxes/arithmetics-common.tmLanguage.json` + `syntaxes/arithmetics.tmLanguage.json`
    (and the same for statemachine).
  - [ ] The selector is now generated into `services_gen.go`. Remove the hand-written
    `sc.Put[core.LanguageSelector]` from their `services.go` so the ID is declared in one place (Q18: agreed).
  - [ ] Delete the hand-written files those replace.
  - [ ] Edit `package.json` by hand: add a `grammars` entry for the common file (`scopeName` + `path`, no `language`).
- [ ] `fastbelt` (the grammar language): keep the hand-written `data/fastbelt.tmLanguage.json`.

## 9. Scaffold (`internal/scaffold`)

- [ ] Add a `gen/main.go.tmpl` using `cmd.BuildContext` (LanguageID and patterns from the template params, plus `TextMateOutput`).
  Point the `go:generate` line in `services.go.tmpl` at it.
- [ ] Remove the hand-written `sc.Put[core.LanguageSelector]` from `services.go.tmpl`. The generated one replaces it (Q18).
- [ ] `package.json.tmpl`: add the grammars entry for the common file.
- [ ] Drop `syntaxes/language.tmLanguage.json.tmpl`, or replace it with a placeholder that the first generate run overwrites.
- [ ] Update `scaffold_test.go`.

## 10. Docs

- [ ] Document TextMate generation: the `BuildContext` fields, scope heuristics and overrides, mode mapping and its limits,
  and the `package.json` snippet. Put it in `doc.go` and/or `grammar/doc.go`.
- [ ] Side note: `workspace/doc.go` still refers to `[FileExtensions]`, which no longer exists. Fix it while you are there.
