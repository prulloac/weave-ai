package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type provider interface {
	chat(ctx context.Context, system, user string) (string, error)
	vision(ctx context.Context, system, prompt string, image []byte, mediaType string) (string, error)
	embed(ctx context.Context, model, text string) ([]float64, error)
}

func newProvider(cfg config, client *http.Client) (provider, error) {
	switch cfg.Provider {
	case "ollama":
		return &ollamaProvider{
			baseURL:     strings.TrimSuffix(cfg.BaseURL, "/"),
			model:       cfg.Model,
			visionModel: cfg.VisionModel,
			client:      client,
		}, nil
	case "openai":
		key := cfg.apiKey()
		if key == "" {
			return nil, fmt.Errorf("%s provider: missing api key", cfg.Provider)
		}
		return &openAIProvider{
			baseURL:     strings.TrimSuffix(cfg.BaseURL, "/"),
			model:       cfg.Model,
			visionModel: cfg.VisionModel,
			key:         key,
			client:      client,
		}, nil
	case "gemini":
		key := cfg.apiKey()
		if key == "" {
			return nil, fmt.Errorf("%s provider: missing api key", cfg.Provider)
		}
		return &geminiProvider{
			baseURL:     strings.TrimSuffix(cfg.BaseURL, "/"),
			model:       cfg.Model,
			visionModel: cfg.VisionModel,
			key:         key,
			client:      client,
		}, nil
	default:
		return nil, fmt.Errorf("%s provider: unsupported", cfg.Provider)
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaProvider struct {
	baseURL     string
	model       string
	visionModel string
	client      *http.Client
}

func (p *ollamaProvider) chat(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model":  p.model,
		"stream": false,
		"format": "json",
		"options": map[string]any{
			"temperature": 0,
		},
		"messages": []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/api/chat", nil, payload, &response); err != nil {
		return "", fmt.Errorf("ollama provider: %w", err)
	}
	return response.Message.Content, nil
}

func (p *ollamaProvider) vision(ctx context.Context, system, prompt string, image []byte, mediaType string) (string, error) {
	payload := map[string]any{
		"model":  p.visionModel,
		"stream": false,
		"format": "json",
		"options": map[string]any{
			"temperature": 0,
		},
		"messages": []any{
			map[string]any{"role": "system", "content": system},
			map[string]any{
				"role":    "user",
				"content": prompt,
				"images":  []string{base64.StdEncoding.EncodeToString(image)},
			},
		},
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/api/chat", nil, payload, &response); err != nil {
		return "", fmt.Errorf("ollama provider: %w", err)
	}
	return response.Message.Content, nil
}

func (p *ollamaProvider) embed(ctx context.Context, model, text string) ([]float64, error) {
	payload := map[string]any{"model": model, "input": text}
	var response struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/api/embed", nil, payload, &response); err != nil {
		return nil, fmt.Errorf("ollama provider: %w", err)
	}
	if len(response.Embeddings) == 0 || len(response.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama provider: empty embedding")
	}
	return response.Embeddings[0], nil
}

type openAIProvider struct {
	baseURL     string
	model       string
	visionModel string
	key         string
	client      *http.Client
}

func (p *openAIProvider) chat(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model":       p.model,
		"temperature": 0,
		"response_format": map[string]any{
			"type": "json_object",
		},
		"messages": []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	headers := map[string]string{"Authorization": "Bearer " + p.key}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/chat/completions", headers, payload, &response); err != nil {
		return "", fmt.Errorf("openai provider: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openai provider: empty response")
	}
	return response.Choices[0].Message.Content, nil
}

func (p *openAIProvider) vision(ctx context.Context, system, prompt string, image []byte, mediaType string) (string, error) {
	imageURL := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(image)
	payload := map[string]any{
		"model":       p.visionModel,
		"temperature": 0,
		"messages": []any{
			map[string]any{"role": "system", "content": system},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": prompt},
					map[string]any{
						"type":      "image_url",
						"image_url": map[string]any{"url": imageURL},
					},
				},
			},
		},
	}
	headers := map[string]string{"Authorization": "Bearer " + p.key}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/chat/completions", headers, payload, &response); err != nil {
		return "", fmt.Errorf("openai provider: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openai provider: empty response")
	}
	return response.Choices[0].Message.Content, nil
}

func (p *openAIProvider) embed(ctx context.Context, model, text string) ([]float64, error) {
	payload := map[string]any{"model": model, "input": text}
	headers := map[string]string{"Authorization": "Bearer " + p.key}
	var response struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/embeddings", headers, payload, &response); err != nil {
		return nil, fmt.Errorf("openai provider: %w", err)
	}
	if len(response.Data) == 0 || len(response.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai provider: empty embedding")
	}
	return response.Data[0].Embedding, nil
}

type geminiProvider struct {
	baseURL     string
	model       string
	visionModel string
	key         string
	client      *http.Client
}

func (p *geminiProvider) chat(ctx context.Context, system, user string) (string, error) {
	endpoint := p.baseURL + "/v1beta/models/" + url.PathEscape(p.model) + ":generateContent"
	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []any{map[string]any{"text": system}},
		},
		"contents": []any{map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": user}},
		}},
		"generationConfig": map[string]any{
			"temperature":      0,
			"responseMimeType": "application/json",
		},
	}
	headers := map[string]string{"x-goog-api-key": p.key}
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := postJSON(ctx, p.client, endpoint, headers, payload, &response); err != nil {
		return "", fmt.Errorf("gemini provider: %w", err)
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini provider: empty response")
	}
	return response.Candidates[0].Content.Parts[0].Text, nil
}

func (p *geminiProvider) vision(ctx context.Context, system, prompt string, image []byte, mediaType string) (string, error) {
	endpoint := p.baseURL + "/v1beta/models/" + url.PathEscape(p.visionModel) + ":generateContent"
	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []any{map[string]any{"text": system}},
		},
		"contents": []any{map[string]any{
			"role": "user",
			"parts": []any{
				map[string]any{"text": prompt},
				map[string]any{
					"inlineData": map[string]any{
						"mimeType": mediaType,
						"data":     base64.StdEncoding.EncodeToString(image),
					},
				},
			},
		}},
		"generationConfig": map[string]any{
			"temperature":      0,
			"responseMimeType": "application/json",
		},
	}
	headers := map[string]string{"x-goog-api-key": p.key}
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := postJSON(ctx, p.client, endpoint, headers, payload, &response); err != nil {
		return "", fmt.Errorf("gemini provider: %w", err)
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini provider: empty response")
	}
	return response.Candidates[0].Content.Parts[0].Text, nil
}

func (p *geminiProvider) embed(ctx context.Context, model, text string) ([]float64, error) {
	endpoint := p.baseURL + "/v1beta/models/" + url.PathEscape(model) + ":embedContent"
	payload := map[string]any{
		"content": map[string]any{
			"parts": []any{map[string]any{"text": text}},
		},
	}
	headers := map[string]string{"x-goog-api-key": p.key}
	var response struct {
		Embedding struct {
			Values []float64 `json:"values"`
		} `json:"embedding"`
	}
	if err := postJSON(ctx, p.client, endpoint, headers, payload, &response); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}
	if len(response.Embedding.Values) == 0 {
		return nil, fmt.Errorf("gemini provider: empty embedding")
	}
	return response.Embedding.Values, nil
}

func postJSON(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	headers map[string]string,
	payload any,
	target any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("status %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("invalid response")
	}
	return nil
}
