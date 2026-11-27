# Task 6: `Plugin.Build`

The design is in [README.md](README.md). Run `go test ./...` when done.

- [ ] Resolve `CommonFile` and `LanguageFiles` against `BuildArtifacts.OutputDir` (absolute paths as is).
- [ ] Errors: empty `CommonFile`; a `LanguageFiles` key that is not a build language; two paths resolving to the same
      file; a language scope equal to the common scope.
- [ ] Languages missing from `LanguageFiles` get no file.
- [ ] `MkdirAll` the parent directories, then write the common file and one file per configured language.
- [ ] Behavior tests on decoded JSON: pushed mode ends at its pop tokens, `mode` switch ends before the parent's
      pop tokens (and warns), comments before keywords, string escapes, scope overrides, unknown override warns.
