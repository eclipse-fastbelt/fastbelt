# Step 2 – Languages artifact

Read `TASK.md` (section "Summary") first.

## Goal

`BuildArtifacts.Languages` contains the languages given to the build context.

## Work

1. In package `pluginapi` (root – it will later point into both `ast` and `tokens`):
   ```go
   type Language struct {
       LanguageID string
       Entry      string   // name of the entry parser rule (as configured)
       Patterns   []string // copied, not aliased
       // EntryMode *tokens.TokenMode  – added in step 4
       // EntryRule *ast.ParserRule    – added in step 7
   }
   ```
   Add `Languages []*Language` to `BuildArtifacts`, in the order of `BuildContext.Languages`.
2. Conversion in `internal/artifacts` from `cmd.Language` data (pass the plain values in; `internal/artifacts`
   must not import `cmd`).
3. Document the fields (Go doc comments).

## Tests

- Single and multiple languages: IDs, entry names, patterns and order are copied correctly.
- `Patterns` slice is a copy (mutating the artifact does not change the build context).
- Empty `Patterns` stays empty/nil consistently (pick one, document it).
- End-to-end via `BuildContext` with the recording plugin, using a multi-language grammar
  (see `internal/languages/multilang/gen/main.go` for a real configuration).
