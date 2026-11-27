# Task 5: Port scopes and rule generation

The design is in [README.md](README.md). Run `go test ./...` when done.

- [ ] Move `textmate_scopes.go` and `textmate_generator.go` into the plugin; replace `grammar.Grammar` by
      `BuildArtifacts` and the grammar name by `Grammar.Name`.
- [ ] Report warnings with `artifacts.Logger.Warn` instead of returning them.
- [ ] Port `textmate_scopes_test.go` and `textmate_generator_test.go` to hand-built artifacts.
