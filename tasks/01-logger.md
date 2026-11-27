# Task 1: Logger in the plugin API

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Add `Logger *slog.Logger` to `pluginapi.BuildArtifacts`; document that it is never nil.
- [x] Add optional `Logger *slog.Logger` to `cmd.BuildContext`.
- [x] Default when nil: a handler writing `Warning: <msg>` to stderr.
- [x] Set it in the artifacts conversion called by `cmd`.
- [x] Tests: a custom logger reaches the plugin; the default is never nil.
- [x] Document it in `pluginapi/doc.go` (new section "Logging").
