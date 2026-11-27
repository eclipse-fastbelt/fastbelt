# Task 6: `Plugin.Build`

The design is in [README.md](README.md). Run `go test ./...` when done.

- [x] Resolve `CommonFile` and `LanguageFiles` against `BuildArtifacts.OutputDir` (absolute paths as is).
- [x] Errors: empty `CommonFile`; a `LanguageFiles` key that is not a build language; two paths resolving to the same
      file; a language scope equal to the common scope.
- [x] Languages missing from `LanguageFiles` get no file.
- [x] `MkdirAll` the parent directories, then write the common file and one file per configured language.
- [x] Behavior tests on decoded JSON: pushed mode ends at its pop tokens, `mode` switch ends before the parent's
      pop tokens (and warns), comments before keywords, string escapes, scope overrides, unknown override warns.

Notes: `Build` also rejects an empty `LanguageFiles` value. It validates and generates all contents before it writes anything, so a failing check or an invalid token
regexp leaves no files behind. Same-file detection compares `filepath.Clean`ed absolute paths (no symlink or
case-insensitivity handling). The scope clash is only checked for languages that get a file. The behavior tests in
`textmate_build_test.go` go through `Build` and read the files back.
