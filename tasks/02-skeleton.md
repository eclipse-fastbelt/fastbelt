# Task 2: Package skeleton

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Create `plugins/textmate` with `doc.go`, `Options`, `New(Options) *Plugin` and `Name() == "textmate"`.
- [x] `New` copies `LanguageFiles` and `Scopes`; fields stay unexported.
- [x] Export `LanguageScopeName(languageID string) string` and `CommonScopeName(grammarName string) string`.
