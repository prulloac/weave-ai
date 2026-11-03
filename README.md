# weave-ai

Reference AI enrichment and reranking extension for [Weave](https://github.com/prulloac/weave).

`weave-ai` is a standalone `weave-<name>` extension. Weave starts it as a subprocess, sends one
`weave.cli/v1` JSON request on stdin and reads one JSON response from stdout. It supports local
Ollama models and OpenAI-compatible or Gemini endpoints.

## Install

User-level installation, no admin or sudo required. Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/prulloac/weave-ai/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/prulloac/weave-ai/main/scripts/install.ps1 | iex
```

The installer downloads the latest release, verifies its SHA-256 checksum and installs to
`~/.local/bin` (Linux/macOS) or `%LOCALAPPDATA%\Programs\weave\bin` (Windows), then adds that
directory to your user PATH.

For a managed or non-standard environment, choose a user-writable directory and skip the profile
edit:

```sh
curl -fsSL https://raw.githubusercontent.com/prulloac/weave-ai/main/scripts/install.sh \
  | sh -s -- --bin-dir "$HOME/bin" --no-modify-path
```

Options: `--version <tag>`, `--bin-dir <dir>`, `--no-modify-path`; environment equivalents
`WEAVE_AI_VERSION`, `WEAVE_AI_BIN_DIR` and `WEAVE_AI_NO_MODIFY_PATH=1`. The PowerShell script
accepts `-Version`, `-BinDir` and `-NoPath`.

`go install github.com/prulloac/weave-ai@latest` is also available for Go users, and every release
includes downloadable archives. Put the binary on `$PATH` or in `<bundle>/.weave/bin/`; a
bundle-local binary wins over `$PATH`.

## Use

```sh
weave-ai --version               # installed version and protocol
weave-ai --describe              # provider, endpoint and consent; never the API key

weave ingest ~/notes --db ~/.weave/index.db --with-enricher ai
weave search ~/notes "token rotation" --rerank ai
```

- Enrichment adds inferred keywords and `custom_metadata` during `ingest`, `rebuild` and `sync`.
- Reranking blends `0.5 * core + 0.5 * rerank`; failures fall back to core order.
- Weave applies the runner timeout (`--timeout`, default `5s`) and reports failures as warnings.

## Configuration

`weave-ai` reads `.weave/ai.yaml` from the current working directory (normally the bundle root) or
the file named by `$WEAVE_AI_CONFIG`:

```yaml
provider: ollama          # ollama | openai | gemini
model: qwen2.5:7b
base_url: http://127.0.0.1:11434
api_key_env: WEAVE_AI_API_KEY
allow_sensitive: false
timeout: 30s
```

| Field | Default | Environment override |
| --- | --- | --- |
| `provider` | `ollama` | `WEAVE_AI_PROVIDER` |
| `model` | `qwen2.5:7b` | `WEAVE_AI_MODEL` |
| `base_url` | provider default | `WEAVE_AI_BASE_URL` |
| `api_key_env` | `WEAVE_AI_API_KEY` | — |
| `allow_sensitive` | `false` | `WEAVE_AI_ALLOW_SENSITIVE` |
| `timeout` | `30s` | `WEAVE_AI_TIMEOUT` |

Provider defaults: Ollama `http://127.0.0.1:11434`, OpenAI `https://api.openai.com/v1`, Gemini
`https://generativelanguage.googleapis.com`. API keys come from the environment variable named by
`api_key_env`; they are never written to config, logs or errors.

`allow_sensitive` is advisory. Weave redacts protected content before IPC, so this setting cannot
recover encrypted, tokenized or hashed text.

## Privacy

Weave redacts encrypted, tokenized and hashed document bodies before sending a request.
`privacy.ai.send: true` in a document's frontmatter opts that document in to sending its body
unredacted. `weave-ai` never receives key material, ciphertext or the index database, and errors
contain only the provider name and HTTP status.

## Protocol

The wire contract is [`weave.cli/v1`](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md).
Enrichment requests carry `doc_id`, `title`, `frontmatter` and `body`; rerank requests carry a query
and candidates with `keyword_score`, `lexical_score` and `fuzzy_score`. Responses return
`inferred_keywords`/`custom_metadata` or `ranked_candidates`.

See the [AI extension specification](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md)
for the full contract.

## Compatibility

`weave-ai` targets `weave.cli/v1`. Release `v1.0.0` was tested with `weave v1.0.0`.

## Development

```sh
./scripts/verify.sh
go build -o weave-ai .
```

Contract fixtures live in `testdata/extension-ipc/` and mirror
[`weave/contract/v1/golden/extension-ipc`](https://github.com/prulloac/weave/tree/main/contract/v1/golden/extension-ipc).
From a workspace checkout, `./scripts/sync-fixtures.sh` refreshes them.

Behavior changes need a spec update in [`weave-docs`](https://github.com/prulloac/weave-docs) first.
See [`AGENTS.md`](AGENTS.md) for the repository workflow.

## License

MIT. See [`LICENSE`](LICENSE).
