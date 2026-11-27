# Step 5 – Source files and locations

Read `TASK.md` (section "Summary") first.

## Goal

The `.fb` source files are available as `*ast.SourceFile`, and there is a `Location` type that AST nodes
(steps 6/7) use to point into them. Source tokens are **not** exposed (decided against, see Summary).

## Work

1. Package `pluginapi/ast`:
   ```go
   type SourceFile struct {
       Path       string // absolute path of the .fb file
       Text       string // full file content
       lineStarts []int  // byte offset of each line start; lineStarts[0] == 0
   }

   // NewSourceFile computes the line index once.
   func NewSourceFile(path, text string) *SourceFile

   // LineColumn returns the 1-based line and the 1-based column in runes of a byte offset.
   // Line breaks are \n, \r\n and \r (same as textdoc). offset is clamped to [0, len(Text)].
   // Works without NewSourceFile too (lineStarts == nil → scan Text, slower but correct).
   func (f *SourceFile) LineColumn(offset int) (line, col int)

   type Location struct {
       Source     *SourceFile // nil for nodes without a single source (merged multi-file Grammar root)
       Start, End int         // byte offsets into Source.Text; Start inclusive, End exclusive
   }

   // Text returns Source.Text[Start:End], or "" if Source == nil.
   func (l Location) Text() string
   ```
   Document clearly: `Location` offsets are **bytes** (what the parser produces, `core.TextRange`, and what slicing
   needs), `LineColumn` columns are **runes**. Line lookup via binary search (`slices.BinarySearch`); do not import
   `textdoc` (its helpers are unexported, UTF-16 based and would pull `lsp` into `pluginapi`).
2. `BuildArtifacts.Sources []*ast.SourceFile`, sorted by path (deterministic, same as grammar loading).
3. Keep the per-file documents from `parseAndMerge` (`cmd/build.go`) available until artifacts are built.
   Record the **node → source file** mapping per file **before** `mergeGrammars` reparents everything into the fake
   `merged.fb` document; steps 6/7 need it to fill `Location.Source`.
4. Create the hook package `internal/artifacts/astinit` (see Summary, "Packages and layering"): only `func`
   variables with `any` parameters (e.g. `SetContainer`, `SetLocation`), imports nothing. `pluginapi/ast` sets them
   in `init()`. Steps 6/7 use them; this step may already wire `SetLocation`.

## Tests

- Single and multi-file grammar: `Sources` sorted by path, `Path` absolute, `Text` equals the file content.
- `LineColumn`: first line/column is (1, 1); multi-line text; `\r\n` counts as one line break; lone `\r` is a line
  break; non-ASCII before the offset counts as one column per rune; offset at `len(Text)`; negative and too large
  offsets are clamped.
- `LineColumn` gives the same results with and without `NewSourceFile` (zero-value `lineStarts`).
- `Location.Text()` returns the slice; `""` for `Source == nil`.
