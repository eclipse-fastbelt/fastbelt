# Step 1 – Plugin API skeleton and plugin invocation

Read `TASK.md` (section "Summary") first; it holds the binding design decisions.

## Goal

Plugins can be registered on `cmd.BuildContext` and are called after code generation with a (still mostly empty) `pluginapi.BuildArtifacts`.

## Work

1. Create the package layout (one-directional imports, see Summary):
   - `pluginapi/tokens` – empty package with `doc.go`
   - `pluginapi/ast` – empty package with `doc.go`
   - `pluginapi/types` – empty package with `doc.go`
   - `pluginapi/pluginapi.go` (package `pluginapi`):
     ```go
     type Plugin interface {
         Name() string
         Build(artifacts BuildArtifacts) error
     }

     type BuildArtifacts struct {
         InputDir  string // absolute
         OutputDir string // absolute; plugins resolve relative paths against it
         Package   string
     }
     ```
   - `pluginapi/doc.go` – package docs: what plugins are, when they run, artifacts are read-only and shared,
     artifacts form a pointer graph and are not meant to be JSON-marshalled, the layering rule
     (`tokens` ← `ast` ← `types` ← `pluginapi`; links only point down, reverse lookups are methods on
     `BuildArtifacts`), plugins are Go-API only (no CLI support).
2. Create `plugins/doc.go` (package `plugins`): home of homegrown Fastbelt plugins; currently empty.
3. Create `internal/artifacts` (package doc only for now): converts internal generator data to `pluginapi` types.
   `pluginapi` must never import internal generator code (the only exception, `internal/artifacts/astinit`,
   follows in step 5).
4. Add `Plugins []pluginapi.Plugin` to `cmd.BuildContext`.
5. Refactor `cmd.Generate` (or split out an internal helper) so `Build()` gets access to the intermediate data
   (expanded grammar, `tokenTypes`, lexer modes, entry rules) needed later for the artifacts. Keep the CLI
   (`cmd/fastbelt`) behaviour unchanged.
6. In `BuildContext.Build()`: after **all** generated files are written, build the artifacts once and call each
   plugin sequentially, passing `BuildArtifacts` by value. Stop on the first error and wrap it:
   `fmt.Errorf("plugin %q: %w", p.Name(), err)`.

## Tests (`cmd/build_test.go`)

- A recording test plugin receives the artifacts with correct `InputDir`, `OutputDir`, `Package`.
- When the plugin runs, the `*_gen.go` files already exist in the output dir.
- Plugins are called in registration order.
- A failing plugin aborts the build, later plugins are not called, the error contains the plugin name and wraps the original (`errors.Is`).
- No plugins configured → build behaves as before.

## Done when

`go build ./... && go test ./cmd/... ./pluginapi/... ./internal/artifacts/...` pass.
