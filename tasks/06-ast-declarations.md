# Step 6 – AST part 1: declarations

Read `TASK.md` (section "Summary") first.

## Goal

A typed, pointer-based mirror of the **merged, infix-expanded** grammar AST for all declaration-level nodes.

## Work

1. Package `pluginapi/ast`, one Go type per grammar interface in `internal/grammar/grammar.fb`, using the
   **exact grammar names**: `Grammar`, `Interface`, `Field`, `FieldType`/`ArrayType`/`ReferenceType`/
   `SimpleType`/`PrimitiveType`, `ParserRule`, `TokenDecl`, `TokenContent`/`RegexpTokenContent`/
   `KeywordTokenContent`, `TokenCommand`, `TokenGroup`, `TokenMode`, `TokenModeMember`/`TokenDeclUsage`/
   `TokenGroupUsage`/`TokenUsage`/`KeywordUsage`/`KeywordSelector`, and the abstract rule interfaces.
   (`Element` and its subtypes, `CompositeRule`, `InfixRule`, `PrecedenceGroup` follow in step 7; until then
   `Body` fields may stay nil.)
2. Conventions:
   - Union / abstract types (`FieldType`, `TokenContent`, `TokenModeMember`, `AbstractRule`, …) are **sealed** Go
     interfaces (unexported marker method); concrete nodes are structs used as pointers.
   - Every `*core.Token` field → `*ast.Token` (step 5). Boolean `?=` fields: `*Token`, nil = false; add a
     helper like `IsEntry() bool`.
   - `Reference[T]` → `*T` (or the sealed interface for abstract targets). Unresolved reference → conversion
     **error** (should never happen since the build aborts on error diagnostics).
   - Common `Node` interface with `Container() Node`, backed by an unexported field set by the converter.
   - `Synthetic bool` on nodes created by infix expansion.
   - `TokenDecl` and `TokenGroup` get `TokenType *tokens.TokenType` (forward link, step 3).
3. Two-pass conversion in `internal/artifacts`: create all nodes, then resolve references.
4. `BuildArtifacts.Grammar *ast.Grammar`.
5. Reverse lookup on the root package (layering rule – `tokens` must not import `ast`):
   `func (a BuildArtifacts) TokenDeclaration(tt *tokens.TokenType) ast.TokenDeclaration`, where
   `ast.TokenDeclaration` is a sealed interface implemented by `*TokenDecl` and `*TokenGroup`; returns nil for EOF
   and for keywords without a `token X: "kw"` declaration. Backed by an unexported map filled by the converter.

## Tests

- **Completeness test**: parse `internal/grammar/grammar.fb`; every interface and every field declared there
  (for the node kinds covered so far) has a counterpart in `pluginapi/ast` (reflection). Elements are allowed to
  be listed as "pending step 7"; the exception list must be empty after step 7.
- References resolved to the right node (`ReturnType`, `Interface.Extends`, `TokenCommand.Mode`, `TokenUsage.TokenRef`, …).
- Containers correct for nested nodes.
- Token positions on names point to the right source token.
- `TokenDecl.TokenType` / `TokenGroup.TokenType` and `TokenDeclaration(tt)` agree; nil for plain keywords and EOF.
- Multi-file grammar: nodes from different files reference each other.
