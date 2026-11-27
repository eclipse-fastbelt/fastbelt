# Task 10: Line anchors in token regexps

Do this after task 9, so that the byte-identical acceptance check (Q17) runs against unchanged output. This is a
behavior change; the generated grammars may change.

Found in the review of task 6. Go parses token regexps with `syntax.Perl` (OneLine), so `^` and `$` become
`OpBeginText` and `OpEndText`. `writeOniguruma` (`plugins/textmate/regexp_utils.go`) emits those as `\A` and `\z`.
vscode-textmate matches one line at a time, including its trailing `\n`, so:

- `//[^\n]*$` becomes `//[^\n]*\z`, which never matches, because `[^\n]*` stops before the `\n`;
- `^#.*` becomes `\A#.*`, which only matches on the first line of a document.

- [ ] Emit `OpBeginText` as `^` and `OpEndText` as `$` (or `(?=\n|\z)`), at least for a plain `^`/`$` (the
      `WasDollar` flag for `$`). Decide what an explicit `\A`/`\z` in the source should become.
- [ ] Tests in `regexp_utils_test.go` for `^`, `$`, `\A`, `\z`, and a behavior test with an anchored line comment.
- [ ] Regenerate the example grammars and review any diff.
