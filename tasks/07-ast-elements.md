# Step 7 – AST part 2: rule bodies, infix and composite rules

Read `TASK.md` (section "Summary") first.

## Goal

The AST mirror is complete: every grammar interface is covered and token-consuming elements link to their token types.

## Work

1. Package `pluginapi/ast`: `Element` (sealed), `Alternatives`, `Group`, `Keyword`, `Assignment`, `Assignable`
   (sealed), `CrossRef`, `RuleCall`, `Action`, `CompositeRule`, `InfixRule`, `PrecedenceGroup`; fill `Body` fields.
2. Token links:
   - `Keyword.TokenType *tokens.TokenType`
   - `RuleCall.TokenType *tokens.TokenType` – set when the called rule is a `TokenDecl` or `TokenGroup`, nil
     for parser/composite rule calls.
   - `CrossRef` consumes through its inner `RuleCall` (document the default-token case if `Rule` is absent).
   - No reverse "usages" list on token types (explicitly decided against).
3. Mark nodes created by `ExpandInfixRules` as `Synthetic`.
4. Add `EntryRule *ast.ParserRule` to `pluginapi.Language`.
5. Remove the "pending step 7" exceptions from the completeness test.

## Tests

- All element kinds converted with correct structure, cardinality and containers.
- `Keyword.TokenType` / `RuleCall.TokenType` point to the right token types (index matches generated constants);
  nil for parser rule calls.
- `Assignment.Property` / `Action.Type` / `Action.Property` / `CrossRef.Type` resolved.
- Infix grammar (`examples/arithmetics/arithmetics.fb`): `InfixRule`, `PrecedenceGroup` present; synthetic nodes flagged.
- Composite rules converted.
- `Language.EntryRule` correct for multi-language grammars.
- Completeness test passes with no exceptions.
