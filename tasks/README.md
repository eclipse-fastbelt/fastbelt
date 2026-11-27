# TextMate generator as a Fastbelt plugin

Status: design agreed in a grilling session on 2026-11-27.
Concrete work items are in one file per task. Do them roughly in this order:

- [1. Logger in the plugin API](01-logger.md)
- [2. Package skeleton](02-skeleton.md)
- [3. Move the regexp utilities](03-regexp-utils.md)
- [4. Build the model from `pluginapi`](04-model.md)
- [5. Port scopes and rule generation](05-scopes-and-rules.md)
- [6. `Plugin.Build`](06-plugin-build.md)
- [7. Remove TextMate from `cmd` and `internal/generator`](07-remove-from-cmd.md)
- [8. Examples and scaffold](08-examples-and-scaffold.md)
- [9. Documentation and cleanup](09-docs-and-cleanup.md)
- [10. Line anchors in token regexps](10-regexp-anchors.md)

## Starting point

- The TextMate generator lives in `internal/generator`: `textmate_generator.go`, `textmate_modes.go`,
  `textmate_scopes.go`, `regexp_utils.go` and their tests.
- `cmd` calls it when `BuildContext.TextMateOutput` is set. `TextMateScopes` overrides scopes. Warnings are printed
  with `fmt.Printf("Warning: …")` (`cmd/build.go`).
- `cmd/build.go` validates language IDs for TextMate (no `/` or `\`, no clash with `<grammar>-common`).
- Golden tests: `cmd/textmate_golden_test.go` with `cmd/testdata/textmate/`.
- `pluginapi` already provides `TokenModes`, `TokenTypes`, `Languages` (with `EntryMode`, `Entry`, `Patterns`) and
  `Grammar.Name`. `internal/artifacts/token_modes.go` builds the modes from the same `generator.LexerModes`, so mode
  names, member order and commands match what the TextMate generator uses today.

## Decisions

### Package and dependencies (Q1, Q2, Q3)

- New public package `plugins/textmate`. It depends only on `pluginapi` and its subpackages, never on
  `internal/...` (the rule in `plugins/doc.go`).
- The mode model (`textMateModel`) is built from `BuildArtifacts.TokenModes`, `TokenTypes` and `Languages` instead of
  `LexerModes`/`GenerateTokenTypes`. Data that turns out to be missing is added to `pluginapi`, not reached through
  internals.
- `regexp_utils.go` (`ToOniguruma`, `SplitDelimiters`, …) and its tests move into the plugin as unexported code.
  TextMate is their only user.

### Keyword names (Q5)

- `pluginapi` names keywords by their value (`(`); the generator uses `grammar.KeywordName` (`LeftParen`).
- The plugin re-implements `KeywordName` so that repository keys and scope override keys stay as they are and the
  output is byte-identical.

### Configuration (Q4, Q7, Q8, Q13)

- `BuildContext.TextMateOutput`, `BuildContext.TextMateScopes`, the matching `GenerateOptions` fields and the TextMate
  branch of `cmd` are removed. The configuration moves into the plugin constructor:

  ```go
  textmate.New(textmate.Options{
      CommonFile:    "…/syntaxes/arithmetics-common.tmLanguage.json",
      LanguageFiles: map[string]string{"arithmetics": "…/syntaxes/arithmetics.tmLanguage.json"},
      Scopes:        map[string]string{"SL_COMMENT": "comment.line"},
  })
  ```

- Fields of the plugin are unexported; `New` copies the maps.
- Paths are relative to `BuildArtifacts.OutputDir` (absolute paths are used as is).
- Empty `CommonFile` → error. A build language missing from `LanguageFiles` → no file for it. A key in
  `LanguageFiles` that is not a language of the build → error. Parent directories are created.
- Plugin name: `textmate`.

### Warnings (Q6, Q9)

- New field `BuildArtifacts.Logger *slog.Logger`, never nil.
- New optional `BuildContext.Logger`. If nil, `cmd` uses a default handler that writes `Warning: <msg>` to stderr.
- The plugin reports its warnings with `Logger.Warn`. Tests pass a handler that collects messages.

### Validation (Q10)

- The old language ID checks in `cmd/build.go` are dropped; with explicit paths they protect nothing.
- `Build` checks instead, and fails with a clear error:
  - two configured paths resolve to the same file (including a language file equal to the common file);
  - a language scope `source.<languageID>` equals the common scope `source.<grammar>-common`.
- These checks run after the Go files are written. That is acceptable.

### Names used by the scaffold (Q11, Q14)

- Scope names stay derived: `source.<languageID>` and `source.<grammar>-common` (lower-cased grammar name).
- The plugin exports `LanguageScopeName(languageID)` and `CommonScopeName(grammarName)`.
- The scaffold passes the file paths into `textmate.New` in its `gen/main.go` template and uses the exported scope
  name functions for `package.json`.

### Tests (Q12, Q15)

- Unit tests build `pluginapi` artifacts (`tokens.TokenMode`, `tokens.TokenType`, `pluginapi.Language`) by hand.
  The regexp tests need no artifacts.
- Behavior tests: generate from small hand-built artifacts, decode the JSON, assert on specific rules (a pushed mode
  ends at its pop tokens, comments before keywords, string escapes, …).
- One integration test: a real `cmd.BuildContext` build with the plugin; files appear at the configured paths,
  warnings reach the logger, includes and scope names match.
- `cmd/textmate_golden_test.go` and `cmd/testdata/textmate/` are deleted.

### Documentation (Q16)

- `plugins/textmate/doc.go` is the reference.
- Root `doc.go`, `README.md`, `grammar/doc.go`, `internal/vscode-extensions/fastbelt/README.md` and the scaffold
  templates are shortened to point to it. `plugins/doc.go` lists the plugin.
- The `TODO/` folder is deleted.

### Acceptance (Q17)

- `go generate` for arithmetics and statemachine produces byte-identical `syntaxes/*.tmLanguage.json`; only their
  `gen/main.go` files change.
