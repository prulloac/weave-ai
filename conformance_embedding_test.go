package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingFixtureConformance(t *testing.T) {
	request := readFixture(t, "embedding-request.json")
	fixture := readFixture(t, "embedding-response.json")

	var expected struct {
		Model  string `json:"model"`
		Dim    int    `json:"dim"`
		Vector string `json:"vector"`
	}
	if err := json.Unmarshal([]byte(fixture), &expected); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	want := decodeVector(t, expected.Vector)
	if len(want) != expected.Dim {
		t.Fatalf("fixture dim %d does not match vector length %d", expected.Dim, len(want))
	}
	providerVector := make([]float64, len(want))
	for i, value := range want {
		providerVector[i] = float64(value)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.URL.Path != "/api/embed" {
			t.Errorf("unexpected path %s", httpRequest.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"embeddings": [][]float64{providerVector},
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
		V      string `json:"v"`
		Model  string `json:"model"`
		Dim    int    `json:"dim"`
		Vector string `json:"vector"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	if output.V != "weave.cli/v1" || output.Model != expected.Model || output.Dim != expected.Dim {
		t.Fatalf("unexpected envelope: %s", stdout)
	}
	got := decodeVector(t, output.Vector)
	if len(got) != len(want) {
		t.Fatalf("vector length %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("vector[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
