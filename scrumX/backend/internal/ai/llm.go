package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LLMClient interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
	GenerateIssues(ctx context.Context, prompt string) ([]IssueDraft, error)
	GenerateSuggestion(ctx context.Context, prompt string) (*Suggestion, error)
}

type noopClient struct{}

func (noopClient) GenerateText(context.Context, string) (string, error) {
	return "", errors.New("llm provider not configured")
}

func (n noopClient) GenerateIssues(ctx context.Context, prompt string) ([]IssueDraft, error) {
	return nil, errors.New("llm provider not configured")
}

func (n noopClient) GenerateSuggestion(ctx context.Context, prompt string) (*Suggestion, error) {
	return nil, errors.New("llm provider not configured")
}

func NewLLMClient(provider, apiKey string) LLMClient {
	provider = strings.ToLower(strings.TrimSpace(provider))
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return noopClient{}
	}
	switch provider {
	case "openai":
		return &openAIClient{
			apiKey: apiKey,
			model:  "gpt-4o-mini",
			client: &http.Client{},
		}
	case "gemini":
		return &geminiClient{
			apiKey: apiKey,
			model:  "gemini-1.5-flash",
			client: &http.Client{},
		}
	default:
		return noopClient{}
	}
}

type openAIClient struct {
	apiKey string
	model  string
	client *http.Client
}

func (c *openAIClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	body := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a concise, structured assistant."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("openai request failed: %s", strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openai empty response")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *openAIClient) GenerateIssues(ctx context.Context, prompt string) ([]IssueDraft, error) {
	content, err := c.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return parseIssueDrafts(content)
}

func (c *openAIClient) GenerateSuggestion(ctx context.Context, prompt string) (*Suggestion, error) {
	content, err := c.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return parseSuggestion(content)
}

type geminiClient struct {
	apiKey string
	model  string
	client *http.Client
}

func (c *geminiClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature": 0.2,
		},
	}
	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gemini request failed: %s", strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini empty response")
	}
	return strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text), nil
}

func (c *geminiClient) GenerateIssues(ctx context.Context, prompt string) ([]IssueDraft, error) {
	content, err := c.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return parseIssueDrafts(content)
}

func (c *geminiClient) GenerateSuggestion(ctx context.Context, prompt string) (*Suggestion, error) {
	content, err := c.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return parseSuggestion(content)
}

func parseIssueDrafts(raw string) ([]IssueDraft, error) {
	clean := extractJSONArray(raw)
	if clean == "" {
		return nil, errors.New("invalid issues response")
	}
	var drafts []IssueDraft
	if err := json.Unmarshal([]byte(clean), &drafts); err != nil {
		return nil, err
	}
	return drafts, nil
}

func parseSuggestion(raw string) (*Suggestion, error) {
	clean := extractJSONObject(raw)
	if clean == "" {
		return nil, errors.New("invalid suggestion response")
	}
	var suggestion Suggestion
	if err := json.Unmarshal([]byte(clean), &suggestion); err != nil {
		return nil, err
	}
	return &suggestion, nil
}

func extractJSONArray(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}
