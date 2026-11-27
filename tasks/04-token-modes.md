# Step 4 – Token modes artifact

Read `TASK.md` (section "Summary") first.

## Goal

`BuildArtifacts.TokenModes` describes the **effective** token modes of the generated lexer (exactly what it runs),
connected to token types via `*tokens.TokenType`, and each language knows its entry mode.

## Facts

- Source: `lexerModes` in `internal/generator/lexer_generator.go` (not exported yet) plus
  `GenerateTokenTypesResult.TokenModes`/`TokenModeOrder`.
- Effective ≠ declared: a grammar without declared modes but several languages gets one synthesized mode per
  entry rule (only reachable tokens); the implicit `default` mode is dropped when the grammar declares modes.
- Keywords are matched before regexp tokens (`modeTokenTypes.Keywords` / `.Tokens`).

## Work

1. Package `pluginapi/tokens`:
   ```go
   type TokenMode struct {
       Index     int                // == generated mode constant
       Name      string
       Synthetic bool               // generated per language, not declared in the grammar
       Members   []*TokenModeMember // lexer priority order: keywords first, then tokens
   }

   type TokenModeMember struct {
       TokenType *TokenType
       Modifier  Modifier      // effective in this mode (shared type, step 3)
       Command   *TokenCommand // nil = no mode change
   }

   type TokenCommand struct {
       Kind   CommandKind // CommandPush | CommandPop | CommandMode ("push"/"pop"/"mode"); shared with ast
       Target *TokenMode // nil for "pop"
   }
   ```
   For synthesized modes, respect the reachability filter (only emitted token types are members;
   hidden/comment tokens and mode switches always included – mirror the generator).
2. `BuildArtifacts.TokenModes []*tokens.TokenMode` with `TokenModes[i].Index == i`.
3. Add `EntryMode *tokens.TokenMode` to `pluginapi.Language`.
4. Export / move `lexerModes` so `internal/artifacts` can reuse it – do **not** duplicate the logic.

## Tests

- Grammar without modes, single language → one `default` mode with all tokens.
- Grammar without modes, several languages → one synthetic mode per entry rule, members restricted to reachable
  tokens, each `Language.EntryMode` correct.
- Grammar with declared modes (fixture `internal/languages/token_modes`): implicit `default` dropped if not
  declared, push/pop/mode commands and targets correct, per-mode modifier overrides correct, keyword-first order.
- Mode indices match the generated lexer's mode constants.
