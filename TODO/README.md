# TextMate generation for multi-file, multi-language grammars

Status: design agreed in a grilling session on 2026-11-17.
Concrete work items are in [tasks.md](tasks.md).

## Starting point

- Branch `lotes/textmate` contains a first draft: `internal/generator/textmate_generator.go`,
  `internal/generator/regexp_utils.go`, their tests, and a `--textMateOut` CLI flag.
  It is based on `4887a5f` and is 47 commits behind `origin/main`.
- Since then `main` gained:
  - token modes (#127)
  - multi-language support (#130)
  - entry rules (#120)
  - semantic tokens (#136)
- A grammar is now a directory of `*.fb` files with the same `grammar` name. They are merged into one
  `grammar.Grammar`. There are no imports.
- Languages are declared in Go through `cmd.BuildContext.Languages`
  (`cmd.Language{Entry, TokenMode, LanguageID, Patterns}`). The `fastbelt generate` CLI supports exactly one entry rule.
- Each language has an entry token mode. It is either declared, or generated per entry rule as
  `TokenMode_<Entry>` with only the reachable tokens (`internal/generator/lexer_generator.go`, `lexerModes()`).
- Known problems in the draft:
  - Language ID and extensions are read from the generator's own container, so they are always `fastbelt` / `.fb`.
  - `Keyword.Text()` is used instead of the unquoted value.
  - `regexp_utils.go` mishandles `?`, panics on `^ $ \b`, and drops the required leading part of `a+`.
  - Punctuation keywords, numbers and other tokens get no scope.
  - Modes are not supported.

## Agreed design

### Overall shape (Q2, Q3, Q10)

One common TextMate file per grammar directory, plus one thin file per language:

```jsonc
// <grammar>-common.tmLanguage.json
{
  "scopeName": "source.<grammar>-common",
  "patterns": [],
  "repository": {
    "Greeting":            { "patterns": [{ "include": "#mode_GreetingMode" }] },   // one per entry rule / language
    "Farewell":            { "patterns": [{ "include": "#mode_FarewellMode" }] },
    "mode_GreetingMode":   { "patterns": [ /* includes of token_*, keywords_*, push rules */ ] },
    "keywords_GreetingMode": { "match": "...", "name": "keyword.control.<grammar>" },
    "token_ID":            { "match": "...", "name": "..." },
    "token_ML_COMMENT_comment": { "begin": "...", "end": "...", "name": "comment.block.<grammar>" }
  }
}
// <languageID>.tmLanguage.json
{ "scopeName": "source.<languageID>", "patterns": [{ "include": "source.<grammar>-common#Greeting" }] }
```

### Naming (Q15)

- `<grammar>` is the lower-cased grammar name.
- Common file: `<grammar>-common.tmLanguage.json`, scope `source.<grammar>-common`.
- Language file: `<languageID>.tmLanguage.json`, scope `source.<languageID>`. This keeps the existing `source.arithmetics` etc.
- Repository keys:
  - `<EntryRuleName>`: one per language
  - `mode_<Mode>`
  - `token_<Name>` and `token_<Name>_comment`
  - `keywords_<Mode>`
- Every scope gets the suffix `.<grammar>`, because the rules are shared between languages.

### Token modes (Q6)

- `push(M)` becomes a `begin`/`end` rule:
  - `begin` is the pushing token
  - `end` is M's `pop` tokens, joined with `|`
  - `patterns` is `include #mode_M`
- `pop` becomes the `end` of the enclosing rule. It is not emitted inside M's own patterns.
- A pushed mode with no `pop` token gets `end: "(?!)"`, so it runs to end of file.
- `mode(M)` has no TextMate equivalent. Approximate it as a rule into M whose `end` is a lookahead `(?=…)`
  on the parent's end pattern. Print a warning when this is ambiguous.

### Modifiers per mode (Q14)

- One repository entry per (token, modifier) combination: `token_X` and `token_X_comment`.
- `hidden` gets no scope from the heuristic, only through an override (changed in task 7: arithmetics' hidden
  `SL_COMMENT` needs `comment.line`).

### Scopes (Q7, Q16, Q17)

- Heuristic defaults:
  - `comment` modifier → `comment.line` / `comment.block` (block when the begin/end extraction finds an end)
  - keywords containing letters → `keyword.control`
  - punctuation and operator keywords → `keyword.operator` / `punctuation`
  - token names like `STRING` → `string.quoted.*`
  - names like `INT|NUMBER|FLOAT` → `constant.numeric`
  - `ID` and `hidden` → no scope
- `token X: "kw"` counts as a keyword.
- Keyword selectors (`keywords /re/`) are resolved through `GenerateTokenTypes`.
- Overrides: `BuildContext.TextMateScopes map[string]string` (token name → scope). The CLI has no TextMate support; everything goes through `BuildContext`.

### Tokens that span lines and regex dialect (Q8)

- TextMate matches one line at a time. For comment and string tokens, extract a literal begin and end and emit `begin`/`end`.
  Everything else gets `match:`.
- Re-emit regexes from the parsed `regexp/syntax` tree in Oniguruma syntax. Do not pass the source text through unchanged.

### Integration (Q5, Q4, Q12, Q13)

- Hook point: `cmd.Generate`, which already receives the merged grammar, entries, entry modes and selectors.
  - New field `BuildContext.TextMateOutput string` (a directory). If it is empty, no TextMate files are generated.
- **Everything is configured in `BuildContext`; there are no CLI flags (Q11, Q11b).**
  - Language ID, entry rule and entry mode for each TextMate file come from `BuildContext.Languages`, the same data
    that generates the `LanguageSelector` in `services_gen.go`.
  - `fastbelt generate` stays single-language and gets no TextMate option. Remove the draft's `--textMateOut` flag.
  - A project that wants TextMate files uses a `gen/main.go` with `cmd.BuildContext`, as in
    `internal/languages/multilang/gen/main.go`, even when it has only one language.
  - Hand-written `LanguageSelector` registrations in the examples and the scaffold template are removed.
    `BuildContext` is the only place a language ID is declared (Q18).
- Always emit the common file plus one file per language, even when there is only one language.
- The generator writes only `*.tmLanguage.json`. Changes to `package.json` are a one-time manual edit:
  - one `contributes.grammars` entry per language
  - one entry for the common file, with `scopeName` and `path` but **no** `language`
- The generated files replace the hand-written ones in the arithmetics and statemachine VS Code extensions.
  `fastbelt.tmLanguage.json` stays hand-written (Q9).

## Resolved: Q11, the source of the language ID

Decision: the language ID comes from `BuildContext.Languages`. There are no CLI flags.
The background below is kept for reference.

### Background

User's answer: "use `service.Get[LanguageID](sc)`, each language should have a service object".

Conflicts found:

1. There is no `workspace.LanguageID` service on `main` any more. It was replaced by one `core.LanguageSelector`
   per container (`language_selector.go:9`, `Select(uri) (int, string)`). It maps URIs to (index, languageID)
   and cannot list the languages it knows.
2. A generate run happens before the user's language package exists or compiles, because it generates that package.
   So there is no service container for that language to read from. The draft did exactly this with
   `grammar.CreateServices()`, and that is why it always got `fastbelt`.
