// Command weave-ai is the reference AI enrichment and reranking extension.
package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
)

const (
	protocolVersion  = "weave.cli/v1"
	maxInput         = 32 << 20
	maxImageDecoded  = 20 << 20
	maxBody          = 64 << 10
	maxText          = 1 << 20
	maxEXIF          = 256 << 10
	maxEmbeddingText = 8 << 10
	maxKeywords      = 32
)

var version = "dev"

type requestEnvelope struct {
	V    string `json:"v"`
	Type string `json:"type"`
}

type enrichmentRequest struct {
	DocID       string         `json:"doc_id"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

type rerankRequest struct {
	Query      string `json:"query"`
	Candidates []struct {
		ID           string  `json:"id"`
		KeywordScore float64 `json:"keyword_score"`
		LexicalScore float64 `json:"lexical_score"`
		FuzzyScore   float64 `json:"fuzzy_score"`
	} `json:"candidates"`
}

type visionRequest struct {
	MediaType string         `json:"media_type"`
	Image     string         `json:"image"`
	Text      string         `json:"text"`
	EXIF      map[string]any `json:"exif"`
	Prompt    string         `json:"prompt"`
}

type visionResponse struct {
	V           string   `json:"v"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Text        string   `json:"text"`
	Keywords    []string `json:"keywords"`
}

type embeddingRequest struct {
	Text      string `json:"text"`
	MediaType string `json:"media_type"`
	Model     string `json:"model"`
}

type embeddingResponse struct {
	V      string `json:"v"`
	Model  string `json:"model"`
	Dim    int    `json:"dim"`
	Vector string `json:"vector"`
}

type enrichmentResponse struct {
	V                string         `json:"v"`
	InferredKeywords []string       `json:"inferred_keywords"`
	CustomMetadata   map[string]any `json:"custom_metadata,omitempty"`
}

type rankedCandidate struct {
	ID          string  `json:"id"`
	RerankScore float64 `json:"rerank_score"`
}

type rerankResponse struct {
	V      string            `json:"v"`
	Ranked []rankedCandidate `json:"ranked_candidates"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			printVersion(os.Stdout)
			return
		case "--describe", "describe":
			os.Exit(describe(os.Stdout, os.Stderr))
		}
	}
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func printVersion(stdout io.Writer) {
	fmt.Fprintf(stdout, "weave-ai %s (%s)\n", version, protocolVersion)
}

func describe(stdout, stderr io.Writer) int {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "weave-ai: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "provider: %s\n", cfg.Provider)
	fmt.Fprintf(stdout, "model: %s\n", cfg.Model)
	fmt.Fprintf(stdout, "vision_model: %s\n", cfg.VisionModel)
	fmt.Fprintf(stdout, "embedding_model: %s\n", cfg.EmbeddingModel)
	fmt.Fprintf(stdout, "endpoint: %s\n", cfg.BaseURL)
	fmt.Fprintf(stdout, "vision: true\n")
	fmt.Fprintf(stdout, "embedding: true\n")
	fmt.Fprintf(stdout, "protocol: %s\n", protocolVersion)
	fmt.Fprintf(stdout, "consent: privacy.ai.send required per document\n")
	fmt.Fprintf(stdout, "api_key_env: %s (value never printed)\n", cfg.APIKeyEnv)
	return 0
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(io.LimitReader(stdin, maxInput+1))
	if err != nil || len(data) > maxInput {
		fmt.Fprintln(stderr, "weave-ai: invalid input")
		return 1
	}
	var envelope requestEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		fmt.Fprintln(stderr, "weave-ai: invalid request")
		return 1
	}
	if envelope.V != "" && envelope.V != protocolVersion {
		fmt.Fprintln(stderr, "weave-ai: unsupported protocol version")
		return 1
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "weave-ai: %s\n", err)
		return 1
	}
	client := &http.Client{Timeout: cfg.Timeout}
	provider, err := newProvider(cfg, client)
	if err != nil {
		fmt.Fprintf(stderr, "weave-ai: %s\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	var response any
	switch envelope.Type {
	case "enrichment_request":
		var request enrichmentRequest
		if err := json.Unmarshal(data, &request); err != nil {
			fmt.Fprintln(stderr, "weave-ai: invalid enrichment request")
			return 1
		}
		response, err = enrich(ctx, provider, request)
	case "vision_request":
		var request visionRequest
		if err := json.Unmarshal(data, &request); err != nil {
			fmt.Fprintln(stderr, "weave-ai: invalid vision request")
			return 1
		}
		response, err = vision(ctx, provider, request)
	case "embedding_request":
		var request embeddingRequest
		if err := json.Unmarshal(data, &request); err != nil {
			fmt.Fprintln(stderr, "weave-ai: invalid embedding request")
			return 1
		}
		response, err = embedding(ctx, provider, cfg, request)
	case "rerank_request":
		var request rerankRequest
		if err := json.Unmarshal(data, &request); err != nil {
			fmt.Fprintln(stderr, "weave-ai: invalid rerank request")
			return 1
		}
		response, err = rerank(ctx, provider, request)
	default:
		fmt.Fprintln(stderr, "weave-ai: unknown request type")
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "weave-ai: %s\n", err)
		return 1
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintln(stderr, "weave-ai: cannot encode response")
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		return 1
	}
	return 0
}

func enrich(ctx context.Context, provider provider, request enrichmentRequest) (any, error) {
	body := request.Body
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	frontmatter, err := json.Marshal(request.Frontmatter)
	if err != nil {
		return nil, fmt.Errorf("invalid frontmatter")
	}
	system := "You enrich documents for a local knowledge database. " +
		"Return ONLY a JSON object with keys inferred_keywords (array of 3-8 lowercase strings) " +
		"and custom_metadata (object, may include a short summary). No prose, no code fences."
	user := fmt.Sprintf("Document id: %s\nTitle: %s\nFrontmatter: %s\nBody:\n%s",
		request.DocID, request.Title, frontmatter, body)
	content, err := provider.chat(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var model struct {
		InferredKeywords []string       `json:"inferred_keywords"`
		CustomMetadata   map[string]any `json:"custom_metadata"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &model); err != nil {
		return nil, fmt.Errorf("invalid model output")
	}
	return enrichmentResponse{
		V:                protocolVersion,
		InferredKeywords: normalizeKeywords(model.InferredKeywords),
		CustomMetadata:   model.CustomMetadata,
	}, nil
}

func vision(ctx context.Context, provider provider, request visionRequest) (any, error) {
	image, err := base64.StdEncoding.DecodeString(request.Image)
	if err != nil {
		return nil, fmt.Errorf("invalid image")
	}
	if len(image) == 0 {
		return nil, fmt.Errorf("invalid image")
	}
	if len(image) > maxImageDecoded {
		return nil, fmt.Errorf("image too large")
	}
	mediaType := request.MediaType
	if mediaType == "" {
		mediaType = "image/jpeg"
	}
	prompt, err := visionPrompt(request)
	if err != nil {
		return nil, err
	}
	system := "You analyze images for a local knowledge database. " +
		"Return ONLY a JSON object with keys title, description, text (optional) and keywords " +
		"(array of 3-8 lowercase strings). No prose, no code fences."
	content, err := provider.vision(ctx, system, prompt, image, mediaType)
	if err != nil {
		return nil, err
	}
	var model struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Text        string   `json:"text"`
		Keywords    []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &model); err != nil {
		return nil, fmt.Errorf("invalid model output")
	}
	text := strings.TrimSpace(model.Text)
	if len(text) > maxText {
		text = text[:maxText]
	}
	return visionResponse{
		V:           protocolVersion,
		Title:       strings.TrimSpace(model.Title),
		Description: strings.TrimSpace(model.Description),
		Text:        text,
		Keywords:    normalizeKeywords(model.Keywords),
	}, nil
}

func visionPrompt(request visionRequest) (string, error) {
	var builder strings.Builder
	builder.WriteString("Describe the image and transcribe any visible text.\n")
	if request.Prompt != "" {
		builder.WriteString(request.Prompt)
		builder.WriteString("\n")
	}
	if request.Text != "" {
		text := request.Text
		if len(text) > maxText {
			text = text[:maxText]
		}
		builder.WriteString("Existing OCR text:\n")
		builder.WriteString(text)
		builder.WriteString("\n")
	}
	if len(request.EXIF) > 0 {
		exif, err := json.Marshal(request.EXIF)
		if err != nil {
			return "", fmt.Errorf("invalid exif")
		}
		if len(exif) > maxEXIF {
			return "", fmt.Errorf("exif too large")
		}
		builder.WriteString("EXIF context:\n")
		builder.Write(exif)
		builder.WriteString("\n")
	}
	return builder.String(), nil
}

func embedding(ctx context.Context, provider provider, cfg config, request embeddingRequest) (any, error) {
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return nil, fmt.Errorf("empty embedding text")
	}
	if len(text) > maxEmbeddingText {
		text = text[:maxEmbeddingText]
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = cfg.EmbeddingModel
	}
	if model == "" {
		return nil, fmt.Errorf("missing embedding model")
	}
	values, err := provider.embed(ctx, model, text)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	raw := make([]byte, 4*len(values))
	for index, value := range values {
		converted := float32(value)
		if math.IsNaN(float64(converted)) || math.IsInf(float64(converted), 0) {
			return nil, fmt.Errorf("invalid embedding")
		}
		binary.LittleEndian.PutUint32(raw[index*4:], math.Float32bits(converted))
	}
	return embeddingResponse{
		V:      protocolVersion,
		Model:  model,
		Dim:    len(values),
		Vector: base64.StdEncoding.EncodeToString(raw),
	}, nil
}

func rerank(ctx context.Context, provider provider, request rerankRequest) (any, error) {
	payload, err := json.Marshal(map[string]any{
		"query":      request.Query,
		"candidates": request.Candidates,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid candidates")
	}
	system := "You rerank search candidates. Return ONLY a JSON object " +
		`{"ranked_candidates":[{"id":"...","rerank_score":0.0-1.0}]} covering every candidate ` +
		"exactly once. Higher scores mean more relevant. No prose, no code fences."
	user := string(payload)
	content, err := provider.chat(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var model struct {
		Ranked []rankedCandidate `json:"ranked_candidates"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &model); err != nil {
		return nil, fmt.Errorf("invalid model output")
	}
	expected := make(map[string]bool, len(request.Candidates))
	for _, candidate := range request.Candidates {
		expected[candidate.ID] = true
	}
	seen := make(map[string]bool, len(model.Ranked))
	ranked := make([]rankedCandidate, 0, len(model.Ranked))
	for _, candidate := range model.Ranked {
		if !expected[candidate.ID] || seen[candidate.ID] {
			return nil, fmt.Errorf("invalid rerank candidates")
		}
		seen[candidate.ID] = true
		score := candidate.RerankScore
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, fmt.Errorf("invalid rerank score")
		}
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		ranked = append(ranked, rankedCandidate{ID: candidate.ID, RerankScore: score})
	}
	if len(ranked) != len(expected) {
		return nil, fmt.Errorf("incomplete rerank response")
	}
	return rerankResponse{V: protocolVersion, Ranked: ranked}, nil
}

func extractJSON(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "```") {
		if index := strings.IndexByte(trimmed, '\n'); index >= 0 {
			trimmed = trimmed[index+1:]
		}
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	}
	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func normalizeKeywords(values []string) []string {
	keywords := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || len(trimmed) > 128 {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		keywords = append(keywords, trimmed)
		if len(keywords) == maxKeywords {
			break
		}
	}
	return keywords
}
