package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "extension-ipc", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestEnrichmentFixtureConformance(t *testing.T) {
	request := readFixture(t, "enrichment-request.json")
	response := readFixture(t, "enrichment-response.json")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %s", httpRequest.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]any{"content": response},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, request, map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}

	var output struct {
		V                string         `json:"v"`
		InferredKeywords []string       `json:"inferred_keywords"`
		CustomMetadata   map[string]any `json:"custom_metadata"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	if output.V != "weave.cli/v1" {
		t.Fatalf("unexpected version: %q", output.V)
	}
	if len(output.InferredKeywords) != 3 || output.InferredKeywords[0] != "sqlite" {
		t.Fatalf("unexpected keywords: %v", output.InferredKeywords)
	}
	if output.CustomMetadata["summary"] != "Selection of pure-Go SQLite." {
		t.Fatalf("unexpected metadata: %v", output.CustomMetadata)
	}
}

func TestRerankFixtureConformance(t *testing.T) {
	request := readFixture(t, "rerank-request.json")
	response := readFixture(t, "rerank-response.json")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", httpRequest.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": response}}},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, request, map[string]string{
		"WEAVE_AI_PROVIDER": "openai",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "test-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}

	var output struct {
		V      string `json:"v"`
		Ranked []struct {
			ID    string  `json:"id"`
			Score float64 `json:"rerank_score"`
		} `json:"ranked_candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	if output.V != "weave.cli/v1" || len(output.Ranked) != 2 {
		t.Fatalf("unexpected response: %s", stdout)
	}
	if output.Ranked[0].ID != "docs/adr-004.md" || output.Ranked[0].Score != 0.95 {
		t.Fatalf("unexpected first candidate: %+v", output.Ranked[0])
	}
	if output.Ranked[1].ID != "docs/adr-001.md" || output.Ranked[1].Score != 0.15 {
		t.Fatalf("unexpected second candidate: %+v", output.Ranked[1])
	}
}
