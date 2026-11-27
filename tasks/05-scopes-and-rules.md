# Task 5: Port scopes and rule generation

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Move `textmate_scopes.go` and `textmate_generator.go` into the plugin; replace `grammar.Grammar` by
      `BuildArtifacts` and the grammar name by `Grammar.Name`.
- [x] Report warnings with `artifacts.Logger.Warn` instead of returning them.
- [x] Port `textmate_scopes_test.go` and `textmate_generator_test.go` to hand-built artifacts.

Notes: the entry points are `generateCommon(artifacts, overrides)` and `generateLanguage(artifacts, language)` in
`plugins/textmate/textmate_generator.go`. The JSON types are unexported (`tmGrammar`, `tmPattern`, `tmCaptures`,
`tmRepository`). Scope names come from `CommonScopeName` and `LanguageScopeName`. The file name helpers
(`TextMateCommonFileName`, `TextMateLanguageFileName`) were not ported, since the paths are configured now. The
checks for an empty entry rule or language ID were dropped, because every `pluginapi.Language` has both. Warning
messages keep their `TextMate: ` prefix. The `internal/generator` copies stay until task 7.
