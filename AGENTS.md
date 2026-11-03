# AGENTS.md

`weave-ai` is the standalone reference AI enrichment and reranking extension for Weave. It is a Go
binary that speaks `weave.cli/v1` over stdin/stdout and is invoked by the `weave` CLI as
`weave-ai`. It is not a library: keep it free of `github.com/prulloac/weave` imports and of direct
access to bundles or index databases.

## Commands

- Build: `go build -o weave-ai .`
- All tests: `go test ./...`
- Race: `go test -race -count=1 -shuffle=on ./...`
- Vet / format: `go vet ./...`; `gofmt -l .` must print nothing
- Local describe: `go run . --describe`
- Full gate: `go test -race -count=1 -shuffle=on ./... && go vet ./... && test -z "$(gofmt -l .)"`
- Docs (optional): `npx --yes markdownlint-cli2 "**/*.md"`

## Layout

```text
main.go             CLI entrypoint, weave.cli/v1 envelopes, enrichment and rerank logic
config.go           .weave/ai.yaml and WEAVE_AI_* configuration
providers.go        Ollama, OpenAI-compatible and Gemini HTTP clients
main_test.go        httptest-based provider and protocol tests
docs/bdd/           behavior scenarios for this binary
README.md           install, usage, configuration and privacy
```

## Contract

- The wire contract is [`weave.cli/v1`](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md),
  not a Go API.
- One request per process: JSON on stdin, JSON on stdout, diagnostics on stderr, exit `0` on success
  and `1` on failure.
- The authoritative behavior spec is
  [`weave-ai-plugin.md`](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md).
  `weave` owns the IPC schema and conformance fixtures per
  [ADR 0003](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0003-extension-protocol-ownership-and-weave-ai-extraction.md).

## Invariants

1. **Spec first:** a behavior or protocol change needs a spec update in `weave-docs` before code.
2. **Behavior preservation:** enrichment and rerank output must remain valid `weave.cli/v1`; core
   validates coverage, clamps scores and falls back on failure.
3. **No secrets in output:** never log or print API keys, document bodies or provider response
   bodies. Errors carry only the provider name and HTTP status.
4. **Privacy:** Weave redacts protected bodies before IPC. Do not add a path that reads bundles,
   keys, ciphertext or the index.
5. **Provider failures degrade:** timeouts, non-2xx responses, malformed JSON and incomplete rerank
   coverage return exit `1`; Weave keeps core behavior.
6. **Configuration precedence:** environment overrides file values; missing config is valid.
7. **Bounded input:** stdin is limited to 1 MiB, enrichment bodies to 64 KiB and keywords to 32.
8. **No core imports:** the binary must build and test from this repository alone.

## Testing quirks

- `main_test.go` uses `httptest` and `t.Setenv`; it never calls a real provider.
- Provider tests assert request paths, authorization headers and response parsing.
- Error-hygiene tests assert that secrets and document bodies never reach stderr.
- Run `go test -race ./...` before opening a pull request.

## Release

- Tags use `vX.Y.Z`; `.github/workflows/release.yml` builds Linux, macOS and Windows archives for
  `amd64` and `arm64`, writes a checksum file and creates a draft GitHub Release.
- Keep the archive and binary name `weave-ai`; Weave discovers extensions by the `weave-` prefix.
- Name the tested `weave` versions in release notes when the protocol or behavior changes.

## Documentation split

- This repository owns install and usage docs (`README.md`) plus extension-side BDD scenarios
  (`docs/bdd/`).
- Core redaction and consent behavior stay in
  [`weave`](https://github.com/prulloac/weave/blob/main/docs/bdd/ai-plugin.md).
- Project specs, architecture and ADRs live in
  [`weave-docs`](https://github.com/prulloac/weave-docs).
