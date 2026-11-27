# Task 3: Move the regexp utilities

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Move `internal/generator/regexp_utils.go` and `regexp_utils_test.go` into `plugins/textmate`, unexported.
- [x] Make sure nothing else in `internal/generator` uses them.

Note: `internal/generator/regexp_utils.go` stays until task 7, because the old TextMate generator in
`internal/generator` still uses it. Its tests moved with the plugin copy.
