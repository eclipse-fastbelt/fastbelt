# Task 8: Examples and scaffold

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] `examples/arithmetics/gen/main.go` and `examples/statemachine/gen/main.go`: register
      `textmate.New(...)` with paths relative to the generated output directory.
- [x] Scaffold: update `templates/gen/main.go.tmpl`, `templates/gen.go.tmpl` and `template_params.go`; use
      `textmate.LanguageScopeName`/`CommonScopeName` instead of the `generator.TextMate*` helpers.
- [x] Update `internal/scaffold/scaffold_test.go` and `internal/e2e_test/scaffold_e2e_test.go`.
- [x] Acceptance: `go generate` for both examples yields byte-identical `syntaxes/*.tmLanguage.json`.

Notes: the examples were already ported in task 7. The scaffold keeps its file names
(`<languageID>.tmLanguage.json`, `<grammar>-common.tmLanguage.json`) and passes them to `textmate.New`; the
`README.md.tmpl` line about the "output directory" was adjusted too.
