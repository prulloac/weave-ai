package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const embeddingInput = `{"v":"weave.cli/v1","type":"embedding_request","text":"hello world","media_type":"text/plain","model":"nomic-embed-text"}`

func decodeVector(t *testing.T, encoded string) []float32 {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode vector: %v", err)
	}
	if len(raw)%4 != 0 {
		t.Fatalf("vector length %d is not float32-aligned", len(raw))
	}
	values := make([]float32, len(raw)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return values
}

func TestOllamaEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/embed" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if payload.Model != "nomic-embed-text" || payload.Input != "hello world" {
			t.Errorf("unexpected payload: %+v", payload)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"embeddings": [][]float64{{0.5, -0.25, 0.125}},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, embeddingInput, map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var response struct {
		V      string `json:"v"`
		Model  string `json:"model"`
		Dim    int    `json:"dim"`
		Vector string `json:"vector"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	values := decodeVector(t, response.Vector)
	if response.V != "weave.cli/v1" || response.Model != "nomic-embed-text" ||
		response.Dim != 3 || len(values) != 3 || values[0] != 0.5 || values[1] != -0.25 {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestOpenAIEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/embeddings" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token: %q", request.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []any{map[string]any{"embedding": []float64{1, 0, 0}}},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, embeddingInput, map[string]string{
		"WEAVE_AI_PROVIDER": "openai",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "test-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var response struct {
		Dim    int    `json:"dim"`
		Vector string `json:"vector"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	values := decodeVector(t, response.Vector)
	if response.Dim != 3 || len(values) != 3 || values[0] != 1 {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestGeminiEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.Path, ":embedContent") {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("x-goog-api-key") != "gem-key" {
			t.Errorf("missing gemini key")
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"embedding": map[string]any{"values": []float64{0, 1, 0}},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, embeddingInput, map[string]string{
		"WEAVE_AI_PROVIDER": "gemini",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "gem-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var response struct {
		Dim    int    `json:"dim"`
		Vector string `json:"vector"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	values := decodeVector(t, response.Vector)
	if response.Dim != 3 || len(values) != 3 || values[1] != 1 {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestEmbeddingRejectsEmptyText(t *testing.T) {
	input := `{"v":"weave.cli/v1","type":"embedding_request","text":"","media_type":"text/plain","model":"nomic-embed-text"}`
	code, stdout, stderr := invokePlugin(t, input, nil)
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure, code=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stderr, "empty") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestEmbeddingRejectsEmptyVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"embeddings": [][]float64{}})
	}))
	defer server.Close()

	code, stdout, _ := invokePlugin(t, embeddingInput, map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure, code=%d stdout=%q", code, stdout)
	}
}

func TestEmbeddingErrorsDoNotLeakText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("provider body super-secret-body"))
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, embeddingInput, map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure, code=%d stdout=%q", code, stdout)
	}
	for _, secret := range []string{"hello world", "super-secret-body"} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
	}
	if !strings.Contains(stderr, "status 502") {
		t.Fatalf("stderr = %q", stderr)
	}
}
