# Task 4: Build the model from `pluginapi`

The design is in [README.md](README.md). Run `go test ./...` when done.

- [ ] Port `textmate_modes.go`: build `textMateModel` from `BuildArtifacts.TokenModes`, `TokenTypes` and
      `Languages` (entry rule name from `Language.Entry`, entry mode from `Language.EntryMode`).
- [ ] Re-implement `grammar.KeywordName` in the plugin; use it as the name of keyword token types without a
      declaration. Token declarations with keyword content keep their declaration name.
- [ ] Map `tokens.Modifier` and `tokens.CommandKind`/`Target` to the model.
- [ ] Port `textmate_modes_test.go` to hand-built `tokens.TokenMode`/`TokenType` values.
- [ ] If anything is missing in `pluginapi`, add it there (with tests and docs) instead of using internals.
