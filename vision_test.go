package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const visionB64 = "ZmFrZS1wbmctYnl0ZXM="

func visionInput() string {
	return `{"v":"weave.cli/v1","type":"vision_request","media_type":"image/png",` +
		`"image":"` + visionB64 + `","text":"Opening hours","exif":{"Make":"Apple"},` +
		`"prompt":"Return JSON with title, description, keywords and optional text."}`
}

const visionModelOutput = `{"title":"Shop","description":"A storefront","text":"Open 9-5","keywords":["shop","sign"]}`

func TestOllamaVision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string   `json:"role"`
				Content string   `json:"content"`
				Images  []string `json:"images"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if payload.Model != "qwen2.5:7b" {
			t.Errorf("model = %q", payload.Model)
		}
		if len(payload.Messages) != 2 || len(payload.Messages[1].Images) != 1 {
			t.Fatalf("unexpected messages: %+v", payload.Messages)
		}
		if payload.Messages[1].Images[0] != visionB64 {
			t.Errorf("image = %q", payload.Messages[1].Images[0])
		}
		if !strings.Contains(payload.Messages[1].Content, "Opening hours") ||
			!strings.Contains(payload.Messages[1].Content, "Apple") {
			t.Errorf("vision prompt missing context: %q", payload.Messages[1].Content)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]any{"content": visionModelOutput},
		})
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, visionInput(), map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var response struct {
		V           string   `json:"v"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Text        string   `json:"text"`
		Keywords    []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode response: %v\n%s", err, stdout)
	}
	if response.V != "weave.cli/v1" || response.Title != "Shop" ||
		response.Description != "A storefront" || response.Text != "Open 9-5" ||
		len(response.Keywords) != 2 {
		t.Fatalf("unexpected response: %s", stdout)
	}
}

func TestOpenAIVision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token: %q", request.Header.Get("Authorization"))
		}
		var payload struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		var imageURL string
		for _, message := range payload.Messages {
			var parts []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			}
			if err := json.Unmarshal(message.Content, &parts); err != nil {
				continue
			}
			for _, part := range parts {
				if part.Type == "image_url" {
					imageURL = part.ImageURL.URL
				}
			}
		}
		if !strings.HasPrefix(imageURL, "data:image/png;base64,") {
			t.Errorf("image url = %q", imageURL)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": visionModelOutput},
			}},
		})
	}))
	defer server.Close()

	code, _, stderr := invokePlugin(t, visionInput(), map[string]string{
		"WEAVE_AI_PROVIDER": "openai",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "test-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}

func TestGeminiVision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.Path, ":generateContent") {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		var payload struct {
			Contents []struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if len(payload.Contents) == 0 || len(payload.Contents[0].Parts) == 0 {
			t.Fatalf("missing contents: %+v", payload)
		}
		var found bool
		for _, part := range payload.Contents[0].Parts {
			if part.InlineData.MimeType == "image/png" && part.InlineData.Data == visionB64 {
				found = true
			}
		}
		if !found {
			t.Errorf("missing inline image: %+v", payload.Contents[0].Parts)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"candidates": []any{map[string]any{
				"content": map[string]any{"parts": []any{map[string]any{"text": visionModelOutput}}},
			}},
		})
	}))
	defer server.Close()

	code, _, stderr := invokePlugin(t, visionInput(), map[string]string{
		"WEAVE_AI_PROVIDER": "gemini",
		"WEAVE_AI_BASE_URL": server.URL,
		"WEAVE_AI_API_KEY":  "gem-key",
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}

func TestVisionRejectsOversizedImage(t *testing.T) {
	raw := make([]byte, maxImageDecoded+1)
	image := base64.StdEncoding.EncodeToString(raw)
	input := fmt.Sprintf(`{"v":"weave.cli/v1","type":"vision_request","media_type":"image/png","image":%q}`, image)

	code, stdout, stderr := invokePlugin(t, input, nil)
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure without output, code=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stderr, "image too large") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestVisionMalformedModelOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]any{"content": "not json"},
		})
	}))
	defer server.Close()

	code, stdout, _ := invokePlugin(t, visionInput(), map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure without output, code=%d stdout=%q", code, stdout)
	}
}

func TestVisionErrorsDoNotLeakImageOrEXIF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("provider body super-secret-body"))
	}))
	defer server.Close()

	code, stdout, stderr := invokePlugin(t, visionInput(), map[string]string{
		"WEAVE_AI_BASE_URL": server.URL,
	})
	if code != 1 || stdout != "" {
		t.Fatalf("expected failure, code=%d stdout=%q", code, stdout)
	}
	for _, secret := range []string{visionB64, "Opening hours", "Apple", "super-secret-body"} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
	}
	if !strings.Contains(stderr, "status 401") {
		t.Fatalf("stderr = %q", stderr)
	}
}
