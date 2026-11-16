![Fastbelt](./fastbelt-full-logo.png)

Fastbelt is a high-performance DSL toolkit for Go with a parser generator and Language Server Protocol (LSP) support.

It is designed for language tooling that needs low latency and high throughput on large workspaces.
According our benchmarks, Fastbelt is between one and two orders of magnitude faster than other state-of-the-art frameworks, such as [Langium](https://langium.org/) and [Xtext](https://eclipse.dev/Xtext/).
For further background and benchmarks, see the [Fastbelt introduction](https://www.typefox.io/blog/fastbelt-introduction/) blog post.

## Installation

### Adding to a Module
Fastbelt ships as a Go module:

```sh
go get typefox.dev/fastbelt@latest
go get -tool typefox.dev/fastbelt/cmd/fastbelt@latest
```

### Global Install

You can also globally install the fastbelt CLI:

```sh
go install typefox.dev/fastbelt/cmd/fastbelt@latest
```

## Quick start

The first step is to write a grammar definition file, e.g. `grammar.fb`, which has a similar format as the grammar language of Langium.

To run the code generator for your grammar definition:

```sh
# globally installed
fastbelt generate ./grammar.fb -o ./
```

```sh
# on demand install
go run typefox.dev/fastbelt/cmd/fastbelt@latest generate ./grammar.fb -o ./
```

This writes generated Go files for services such as lexer, parser, linker, and type definitions.

Typically you will want to run generation using `go generate`.
Add a directive to some file in your module (assumes install with `go tool`):

```go
//go:generate go tool typefox.dev/fastbelt/cmd/fastbelt generate ./grammar.fb -o ./
```

## Scaffolding

To bootstrap a **new Go module** for a language, run:

```sh
fastbelt scaffold --module example.com/you/mylang --language "MyLanguage" --vscode
```

That creates a directory named after the last segment of `--module` (here `./mylang`) in the current working directory, runs `go mod init`, pulls in fastbelt as a library and tool dependency, and runs `go generate` and `go mod tidy` so generated parser, lexer, linker, and supporting files are ready to use.

In the generated package directory you get:

- `services.go` with the `go:generate` directive for grammar-based code generation and customization points for generated services
- `<language-id>.fb` as the initial grammar definition
- `cmd/<language-id>-lsp/main.go` as the LSP server entrypoint

Pass `--vscode` to also add a `vscode-extension` subfolder for editor integration.

To add a language package to an **existing** module, omit `--module` and use `--package` (or `-p`) instead; see `fastbelt scaffold -h` for full usage.

## Examples

This repository contains examples in the `examples` directory. Currently, this includes:

1. `examples/statemachine` is a simple example showing an implementation of Martin Fowler's [State Machine DSL]().
   It contains a showcase of how to compile a Fastbelt based language server into WebAssembly.
2. `examples/arithmetics` shows how to write a language that contains complex infix expressions.

For editor integration, see the VS Code extensions in `internal/vscode-extensions/`.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution guidelines.

## License

Fastbelt is licensed under the [MIT License](./LICENSE).

## Citation

You can cite Fastbelt using the following bibtex entry:

```
@inproceedings{
    10.1145/3837062.3838878,
    author = {Sujew, Mark and Smyth, Steven},
    title = {Fastbelt: An Architecture for Efficient Language Engineering},
    year = {2026},
    isbn = {9798400729034},
    publisher = {Association for Computing Machinery},
    address = {New York, NY, USA},
    url = {https://doi.org/10.1145/3837062.3838878},
    doi = {10.1145/3837062.3838878},
    booktitle = {Proceedings of the ACM/IEEE 29th International Conference on Model Driven Engineering Languages and Systems},
    pages = {249–253},
    numpages = {5},
    location = {M{\'a}laga, Spain},
    series = {MODELS Companion '26}
}
```
