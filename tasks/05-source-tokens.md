# Step 5 – Source files and source tokens

Read `TASK.md` (section "Summary") first.

## Goal

The `.fb` source tokens are available as `*ast.Token` so AST nodes (steps 6/7) can reference them instead of
`*core.Token`.

## Work

1. Package `pluginapi/ast`:
   ```go
   type SourceFile struct {
       Path   string   // absolute path of the .fb file
       Text   string   // full file content
       Tokens []*Token // all tokens incl. hidden and comment tokens, in source order
   }

   type Position struct{ Offset, Line, Column int } // 0-based like LSP; Offset in bytes

   type Token struct {
       Image     string
       Source    *SourceFile // nil if Synthetic
       Index     int         // position in Source.Tokens; -1 if Synthetic
       Start     Position
       End       Position
       Synthetic bool
   }

   func (t *Token) Text() string // nil-safe, "" for nil
   ```
   Do not copy `core.Token.Type` (it is a token type of the Fastbelt grammar language, not of the user language).
2. `BuildArtifacts.Sources []*ast.SourceFile`, sorted by path (deterministic, same as grammar loading).
3. The loader must keep the per-file `core.Document`s (`Tokens` and `Comments`) available until artifacts
   are built; merge regular and comment tokens by offset.
   **Hidden tokens (whitespace etc.):** the lexer drops them, so they are **not** included. Do not change the lexer.
   Instead add `Text string` (full file content) to `SourceFile`, so a plugin can recover anything between two
   tokens via their offsets. Document this. If a plugin later really needs hidden tokens as `*Token`s, that can be
   added without breaking the API (open question, deliberately postponed).
4. Provide an internal mapping `*core.Token → *ast.Token` in `internal/artifacts` for steps 6/7; tokens created
   by `NewSyntheticToken` / infix expansion map to a fresh `Synthetic` token.

## Tests

- Single and multi-file grammar: files sorted, tokens complete and in order, `Index` matches slice position.
- Positions (offset/line/column) correct, including multi-line comments and non-ASCII text.
- Comment tokens present; hidden tokens absent; the gap between two tokens is recoverable from `Text` via offsets.
- Synthetic token: `Source == nil`, `Index == -1`; `(*Token)(nil).Text() == ""`.
