# Task 7: Remove TextMate from `cmd` and `internal/generator`

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Remove `TextMateOutput` and `TextMateScopes` from `BuildContext` and `GenerateOptions`, and the TextMate
      branch and language ID checks from `cmd/build.go`.
- [x] Delete the TextMate files left in `internal/generator`, including `regexp_utils.go`.
- [x] Delete `cmd/textmate_golden_test.go`, `cmd/testdata/textmate/` and the TextMate tests in `cmd/build_test.go`.
- [x] Add one integration test in `plugins/textmate` (external test package) that runs `cmd.BuildContext` with the
      plugin: files at the configured paths, warnings in the logger, include and scope names consistent.

Notes: `examples/*/gen/main.go` were ported to the plugin here (item 1 of task 8) so the module compiles; `go generate`
output is byte-identical. The scaffold template still emits `TextMateOutput` until task 8.
