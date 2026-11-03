# BDD: Weave AI Extension

Behavior cases for the `weave-ai` subprocess. Core redaction and per-document consent are owned by
[`weave`](https://github.com/prulloac/weave/blob/main/docs/bdd/ai-plugin.md).

**Spec reference:** [weave-ai-plugin.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md)
**Go tests:** `main_test.go`

---

## Feature: Plugin Protocol

### Scenario: Ollama enrichment

- **Given** an Ollama-compatible endpoint returning JSON content
- **When** the plugin receives an `enrichment_request`
- **Then** it emits `weave.cli/v1` inferred keywords
- **And** exits `0`

### Scenario: OpenAI-compatible rerank

- **Given** an OpenAI-compatible endpoint
- **When** the plugin receives a `rerank_request`
- **Then** it emits ranked candidates with clamped scores

### Scenario: Gemini enrichment

- **Given** a Gemini endpoint and API key
- **When** the plugin receives an enrichment request
- **Then** it emits inferred keywords

### Scenario: Malformed model output fails

- **When** the provider returns non-JSON content
- **Then** the plugin exits `1`
- **And** writes no response to stdout

### Scenario: Unknown request type fails

- **When** stdin carries an unknown `type`
- **Then** the plugin exits `1`

### Scenario: Errors do not leak keys or bodies

- **Given** a provider returning an error body containing secrets
- **When** the call fails
- **Then** stderr contains only provider and status
- **And** never the API key or document body

### Scenario: Invalid configuration fails

- **When** the provider is unknown or the URL is invalid
- **Then** the plugin exits `1`
