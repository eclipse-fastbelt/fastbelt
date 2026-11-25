# Step 3 – Token types artifact

Read `TASK.md` (section "Summary") first.

## Goal

`BuildArtifacts.TokenTypes` describes every token type of the generated lexer, indexed by its runtime ID.

## Facts

- Source: `generator.GenerateTokenTypes` (`internal/generator/lexer_util.go`), computed **after**
  `grammar.ExpandInfixRules` (so infix-synthesized operator groups are included).
- Indices are dense: 0 = EOF, then keywords, then regexp token declarations, then token groups.
  A keyword-content declaration (`token A: "a"`) shares the keyword's index – one index, several names.

## Work

1. Package `pluginapi/tokens`:
   ```go
   type TokenKind int // EOF, Keyword, Regexp, Group

   type TokenType struct {
       Index    int          // runtime ID (== core.TokenType.Id); artifacts.TokenTypes[Index] == this
       Name     string       // primary name, e.g. "ID", "hello", "Cardinality"
       Aliases  []string     // other names sharing this index, e.g. "A" for keyword "a"
       Kind     TokenKind
       Keyword  string       // Kind == Keyword: the keyword value (unquoted)
       Regexp   string       // Kind == Regexp: regexp source without the surrounding slashes
       Members  []*TokenType // Kind == Group: direct members
       Modifier string       // declared modifier: "", "hidden", "comment"
       Synthetic bool        // created by infix expansion
   }
   ```
   No compiled automata / rune sets.
2. `BuildArtifacts.TokenTypes []*tokens.TokenType` with `TokenTypes[i].Index == i`.
3. Export from `internal/generator` only what `internal/artifacts` needs (prefer small accessor functions
   over exporting the internal lookup structs wholesale).
4. Decide and document which name is primary when keyword and declaration share an index
   (recommendation: the declaration name, keyword value as alias), and how a keyword's `Name` looks.

## Tests (`internal/artifacts`, explicit assertions, no golden files)

- EOF at index 0; indices dense; `TokenTypes[i].Index == i`.
- Keyword, regexp token, hidden/comment token, token group (incl. nested group → `Members`).
- Shared index of `"a"` and `token A: "a"` → one entry, alias present.
- Infix grammar (e.g. `examples/arithmetics/arithmetics.fb`): synthesized operator group present and `Synthetic`.
- Consistency check: for a fixture language, indices match the generated `*_Idx` constants / `core.TokenType.Id`
  (e.g. compare with `internal/languages/token_groups`).
