# Task 4: Build the model from `pluginapi`

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Port `textmate_modes.go`: build `textMateModel` from `BuildArtifacts.TokenModes`, `TokenTypes` and
      `Languages` (entry rule name from `Language.Entry`, entry mode from `Language.EntryMode`).
- [x] Re-implement `grammar.KeywordName` in the plugin; use it as the name of keyword token types without a
      declaration. Token declarations with keyword content keep their declaration name.
- [x] Map `tokens.Modifier` and `tokens.CommandKind`/`Target` to the model.
- [x] Port `textmate_modes_test.go` to hand-built `tokens.TokenMode`/`TokenType` values.
- [x] If anything is missing in `pluginapi`, add it there (with tests and docs) instead of using internals.

Notes: the model lives in `plugins/textmate/modes.go` and takes `TokenModes` and `Languages`. The token types come
through the mode members, so there is no separate `TokenTypes` parameter. Nothing had to be added to `pluginapi`:
an undeclared keyword is a `KindKeyword` token type without `Aliases`. The old tests for keyword selectors, unknown
target modes and declared language modes were testing `LexerModes`, and `internal/artifacts` covers that now. The
port tests the model with hand-built modes. The old test without entry rules was dropped as well, because every
`pluginapi.Language` has an entry. The old generator then skipped the language with `Entry == ""`, so its output does
not change.

A keyword with several token declarations takes the name of the first one (top-level first, then grammar order). The
old model picked one of them at random through map iteration. So the output is byte-identical only for grammars with
at most one declaration per keyword, and the examples are such grammars.
