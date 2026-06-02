# Contributing to Clair

Thanks for your interest in contributing. This document covers how to get set up, the conventions we follow, and how to submit changes.

## Getting started

```
git clone git@github.com:trendvidia/clair.git
cd clair
go build ./...
go test ./...
```

Requires Go 1.26 or newer.

To install the CLI locally:

```
go install ./cmd/kbtool
```

## Project layout

See the [README](README.md#layout) for a tour of the packages. Briefly:

- `kb/` — the two-tier knowledge base (framework + project) on top of Bleve.
- `assistant/` — orchestration layer combining KB search with optional LLM features.
- `llm/` — provider-agnostic LLM interface and adapters.
- `indexer/`, `parser/`, `privacy/` — supporting subsystems.
- `cmd/kbtool/` — CLI entry point.

## Making changes

1. Open an issue first for anything non-trivial — a quick discussion saves rework.
2. Branch from `main` (e.g. `git checkout -b fix-watcher-debounce`).
3. Keep commits focused. Prefer several small commits over one large one.
4. Run `go test ./...` and `go vet ./...` before pushing.
5. Open a PR against `main`. Link the issue it addresses, if any.

## Style

- Follow standard Go formatting (`gofmt` / `goimports`). CI rejects unformatted code.
- Every exported symbol must have a doc comment. Package-level docs go on one file per package.
- Errors returned from this module are prefixed with `clair:` and wrapped with `%w` so callers can unwrap.
- Keep public APIs minimal — prefer adding behavior through `Option` functions (see `assistant.Option`) over expanding struct fields.

## Tests

- Unit tests live next to the code (`kb_test.go`, `privacy_test.go`, etc.).
- Parser fixtures go under `parser/<lang>parser/testdata/`.
- Don't mock the Bleve index in tests — open a real one in a `t.TempDir()`. The index is fast enough and mocks have hidden us from real bugs before.

## Documentation

- User-facing changes update `README.md`.
- Behavioral changes that affect contributors update this file.
- Index-affecting changes: run `kbtool validate` against a representative index and include the output in the PR description if symbol coverage drops.

## Reporting issues

Open a GitHub issue with:

- Clair version (commit SHA is fine).
- Go version (`go version`).
- A minimal reproduction or, for indexing/search bugs, the query and a sketch of the source the index was built from.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
