# BDD: Weave AI Extension

Behavior cases for the `weave-ai` subprocess. Core redaction and per-document consent are owned by
[`weave`](https://github.com/prulloac/weave/blob/main/docs/bdd/ai-plugin.md).

**Spec reference:** [weave-ai-plugin.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md)
**Go tests:** `main_test.go`

---

## Feature: Vision

### Scenario: Ollama vision round-trip

- **Given** an Ollama-compatible endpoint returning vision JSON
- **When** the plugin receives a `vision_request`
- **Then** the request includes the image and OCR context
- **And** the response contains title, description, text and keywords
- **And** it exits `0`

### Scenario: OpenAI-compatible vision round-trip

- **Given** an OpenAI-compatible endpoint
- **When** the plugin receives a vision request
- **Then** the request uses a data URL with the stored media type
- **And** the response is parsed as JSON

### Scenario: Gemini vision round-trip

- **Given** a Gemini endpoint and API key
- **When** the plugin receives a vision request
- **Then** the request uses inline image data with its MIME type

### Scenario: Oversized image fails before the provider

- **Given** an image above 20 MiB after base64 decoding
- **When** the plugin receives a vision request
- **Then** it exits `1` with an image-size error
- **And** no provider call is made

### Scenario: Vision errors do not leak image or EXIF

- **Given** a provider returning an error body
- **When** vision fails
- **Then** stderr contains only provider and status
- **And** never the image, OCR text or EXIF

---

## Feature: Embeddings

### Scenario: Ollama embeddings

- **Given** an Ollama-compatible endpoint
- **When** the plugin receives an `embedding_request`
- **Then** it calls the embed endpoint with the model and text
- **And** returns a base64 float32 little-endian vector with its dimension

### Scenario: OpenAI-compatible embeddings

- **Given** an OpenAI-compatible endpoint
- **When** the plugin receives an embedding request
- **Then** it calls `/embeddings` with the bearer token
- **And** returns the vector

### Scenario: Gemini embeddings

- **Given** a Gemini endpoint and API key
- **When** the plugin receives an embedding request
- **Then** it calls `:embedContent`
- **And** returns the vector

### Scenario: Empty embedding text fails

- **When** the request text is empty
- **Then** the plugin exits `1`
- **And** writes no response

### Scenario: Empty or invalid vectors fail

- **Given** a provider returning no vector or non-finite values
- **Then** the plugin exits `1`
- **And** writes no response

### Scenario: Embedding errors do not leak text

- **Given** a provider returning an error body
- **When** embedding fails
- **Then** stderr contains only provider and status
- **And** never the input text

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

### Scenario: Describe reports capabilities

- **When** I run `weave-ai --describe`
- **Then** it prints the provider, model, vision model and embedding model
- **And** it reports `vision: true`, `embedding: true` and the protocol
- **And** it never prints API key values

### Scenario: Vision and embedding model overrides

- **Given** a configuration file with `vision_model` and `embedding_model`
- **When** `WEAVE_AI_VISION_MODEL` or `WEAVE_AI_EMBEDDING_MODEL` is set
- **Then** the environment value wins
