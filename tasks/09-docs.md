# Step 9 – Documentation and wrap-up

Read `TASK.md` (section "Summary") first.

## Work

1. `pluginapi/doc.go`: full guide – writing a plugin, registering it on `cmd.BuildContext` (Example 1 from
   `TASK.md`), execution order and error handling, relative paths resolve against `OutputDir`, artifact overview
   (languages, token types, token modes, sources, AST, types) and how they link, read-only/shared contract,
   pointer graph (no JSON marshalling), layering rule, Go-API only.
2. Package docs for `pluginapi/tokens`, `pluginapi/ast`, `pluginapi/types`, `plugins`.
3. Mention the plugin system in the root `doc.go` and the `cmd` package doc.
4. Add `pluginapi/doc.go` and `plugins/doc.go` to the list in `AGENTS.md`.
5. A runnable `Example` test in `pluginapi` showing a minimal plugin (e.g. listing keywords per language).
6. Final check: `go vet ./...`, `go test ./...`, linter per `CONTRIBUTING.md`.

Out of scope: the TextMate plugin from Example 1 (separate follow-up task).
