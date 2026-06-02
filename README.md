# Clair

Clair is a knowledge base assistant for code. It builds a searchable index of a framework's API plus the user's own project, and exposes it through a plain search interface and an optional LLM-powered Q&A layer.

The built-in search works offline with no external dependencies. The AI layer is opt-in and pluggable across providers (Anthropic, Ollama, OpenAI-compatible).

## Layout

```
assistant/   Orchestration combining the KB with optional LLM features
cmd/kbtool   CLI for building, inspecting, searching, and validating indexes
indexer/     Full and incremental project indexing with file watching
kb/          Two-tier (framework + project) Bleve-backed knowledge base
llm/         Provider-agnostic LLM interface + Anthropic/Ollama/OpenAI adapters
parser/      Source parser interface and Go implementation
privacy/     Data-flow policy for filtering what reaches remote LLMs
```

## Concepts

- **Framework KB** — a read-only, pre-built index shipped with the IDE.
- **Project KB** — a read-write index of the user's source, kept in sync by the indexer's file watcher.
- **Merged KB** — searches across both tiers and merges results.
- **Privacy policy** — gates what file-sourced content can be sent to remote LLMs; defaults to filtering out env files, credentials, and keys.

## kbtool

```
kbtool build    --source <dir> --module <path> --output <path>
kbtool inspect  <index-path>
kbtool search   <index-path> <query>
kbtool validate <index-path>
```

`build` scans a source tree with the Go parser and produces a Bleve index. `validate` reports symbols missing documentation and exits non-zero if any are found.

## Library usage

```go
fwKB, _ := kb.OpenFramework("/path/to/framework.bleve")
projKB, _ := kb.OpenProject("/path/to/project.bleve")

a := assistant.New(fwKB, projKB,
    assistant.WithProvider(anthropic.New(apiKey)),
    assistant.WithPrivacy(privacy.DefaultPolicy()),
)
defer a.Close()

// Offline search:
results, _ := a.Builtin.Search("http handler", 10)

// AI-powered, with KB results injected as context:
stream, _ := a.AI.Ask(ctx, "how do I register a route?", assistant.EditorContext{
    CurrentFile: "main.go",
    Language:    "go",
})
```

## Build

```
go build ./...
go test ./...
go install ./cmd/kbtool
```

Requires Go 1.26 or newer.
