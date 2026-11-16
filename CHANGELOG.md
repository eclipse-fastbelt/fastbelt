# v0.2.0 (Nov. 2026)

## Highlights

- **Multi-language support**: a workspace can now host several languages in one build, and a grammar can be split across multiple `.fb` files in one directory ([#130](https://github.com/eclipse-fastbelt/fastbelt/pull/130))
- **Infix rules**: declare operator precedence and associativity directly in the grammar ([#134](https://github.com/eclipse-fastbelt/fastbelt/pull/134))
- **Token modes**: context-dependent lexing with explicit token precedence per mode ([#127](https://github.com/eclipse-fastbelt/fastbelt/pull/127))
- **JSON serialization**: generated `MarshalJSONTo` / `UnmarshalJSONFrom` for AST types, including cross-references ([#98](https://github.com/eclipse-fastbelt/fastbelt/pull/98))
- **Much broader LSP coverage**: semantic tokens, plus many new provider services ([#136](https://github.com/eclipse-fastbelt/fastbelt/pull/136), [#125](https://github.com/eclipse-fastbelt/fastbelt/pull/125), [#191](https://github.com/eclipse-fastbelt/fastbelt/pull/191))
- **Faster and leaner**: fewer allocations across the lexer, parser, AST and linker

## Breaking changes

The release contains too many breaking changes to reasonably reference in the changelog. This is a selection of important breaking changes:

- Go 1.27 is now required, for `encoding/json/v2` ([#98](https://github.com/eclipse-fastbelt/fastbelt/pull/98)) and generic service container methods ([#165](https://github.com/eclipse-fastbelt/fastbelt/pull/165)).
- Entry rules must be marked with the `entry` keyword; the first parser rule is no longer the implicit entry rule ([#130](https://github.com/eclipse-fastbelt/fastbelt/pull/130))
- Internal ranges use byte offsets instead of line/column positions ([#128](https://github.com/eclipse-fastbelt/fastbelt/pull/128))
- `TypeId` was removed from `Token` ([#135](https://github.com/eclipse-fastbelt/fastbelt/pull/135))
- `SetContainer`, `ForEachNode` and `ForEachReference` now carry the containing field name and index ([#124](https://github.com/eclipse-fastbelt/fastbelt/pull/124))
- Service container functions are now generic methods ([#165](https://github.com/eclipse-fastbelt/fastbelt/pull/165))
- Validation functions receive the service container ([#151](https://github.com/eclipse-fastbelt/fastbelt/pull/151))
- Updated to go-lsp v0.1.0, which contains breaking changes on its own ([#148](https://github.com/eclipse-fastbelt/fastbelt/pull/148))

## Grammar language

- Add support for Langium v4 style infix rules ([#134](https://github.com/eclipse-fastbelt/fastbelt/pull/134))
- Add support for ANTLR style token modes ([#127](https://github.com/eclipse-fastbelt/fastbelt/pull/127))
- Grammars can now be split across multiple `.fb` files in one directory ([#130](https://github.com/eclipse-fastbelt/fastbelt/pull/130))
- Improvements to unicode case folding for the generated lexer code ([#154](https://github.com/eclipse-fastbelt/fastbelt/pull/154))

## Language server

- Add support for semantic tokens requests ([#136](https://github.com/eclipse-fastbelt/fastbelt/pull/136))
- New provider interfaces for call hierarchy, type hierarchy, code actions, code lens, commands, declaration, type definition, implementation, document links, inlay hints and signature help ([#125](https://github.com/eclipse-fastbelt/fastbelt/pull/125))
- LSP 3.18 dynamic text document content ([#191](https://github.com/eclipse-fastbelt/fastbelt/pull/191))
- Improved completion for cross-references ([#155](https://github.com/eclipse-fastbelt/fastbelt/pull/155))
- Per-document read locks, so requests no longer wait on unrelated documents ([#147](https://github.com/eclipse-fastbelt/fastbelt/pull/147))
- Support cancellation of in-flight lexing and parsing ([#156](https://github.com/eclipse-fastbelt/fastbelt/pull/156))
- Ensure document changes are always kept in sync ([#189](https://github.com/eclipse-fastbelt/fastbelt/pull/189))

## AST and runtime

- Locate AST nodes by path with `PathOf` and `Resolve` ([#124](https://github.com/eclipse-fastbelt/fastbelt/pull/124))
- JSON import and export of ASTs ([#98](https://github.com/eclipse-fastbelt/fastbelt/pull/98))
- Fixed the order of the workspace builder reset ([#132](https://github.com/eclipse-fastbelt/fastbelt/pull/132))

## Tooling

- `fastbelt generate --railroad` writes an SVG railroad diagram per rule; the VS Code extension shows the same diagram on hover in `.fb` files ([#152](https://github.com/eclipse-fastbelt/fastbelt/pull/152))
- Programmatic builds are now supported through the `cmd` package ([#130](https://github.com/eclipse-fastbelt/fastbelt/pull/130))
- Package documentation for `parser` and `server` ([#129](https://github.com/eclipse-fastbelt/fastbelt/pull/129))

## Performance

- Fewer goroutines per build, and `parallel.ForEach` runs inline when one worker suffices ([#126](https://github.com/eclipse-fastbelt/fastbelt/pull/126), [#168](https://github.com/eclipse-fastbelt/fastbelt/pull/168))
- Fewer allocations in the parser and AST, with deferred AST node allocation ([#133](https://github.com/eclipse-fastbelt/fastbelt/pull/133), [#142](https://github.com/eclipse-fastbelt/fastbelt/pull/142), [#157](https://github.com/eclipse-fastbelt/fastbelt/pull/157))
- Smaller `Reference[T]` struct and faster linking ([#166](https://github.com/eclipse-fastbelt/fastbelt/pull/166), [#144](https://github.com/eclipse-fastbelt/fastbelt/pull/144))
- AST node iterators short-circuit earlier ([#153](https://github.com/eclipse-fastbelt/fastbelt/pull/153))
- The lexer consumes self-looping ASCII runs with a table lookup ([#169](https://github.com/eclipse-fastbelt/fastbelt/pull/169))
- The ATN is generated as tables instead of code ([#179](https://github.com/eclipse-fastbelt/fastbelt/pull/179))
- Cheaper LL(*) prediction: one DFA per prediction mode, and lazily built outer contexts and merge caches ([#171](https://github.com/eclipse-fastbelt/fastbelt/pull/171), [#173](https://github.com/eclipse-fastbelt/fastbelt/pull/173), [#175](https://github.com/eclipse-fastbelt/fastbelt/pull/175), [#176](https://github.com/eclipse-fastbelt/fastbelt/pull/176), [#177](https://github.com/eclipse-fastbelt/fastbelt/pull/177))
- Line offsets of a text document are computed on first use ([#170](https://github.com/eclipse-fastbelt/fastbelt/pull/170))

# v0.1.0 (Jun. 2026)

Initial release of Fastbelt.
