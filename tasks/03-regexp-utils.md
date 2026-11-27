# Task 3: Move the regexp utilities

The design is in [README.md](README.md). Run `go test ./...` when done.

- [ ] Move `internal/generator/regexp_utils.go` and `regexp_utils_test.go` into `plugins/textmate`, unexported.
- [ ] Make sure nothing else in `internal/generator` uses them.
