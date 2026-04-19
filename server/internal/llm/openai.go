package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"smem/apps/server/internal/retry"
)

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Retry      retry.Policy
}

type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	retry      retry.Policy
}

func NewOpenAIProvider(cfg Config) *OpenAIProvider {
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = retry.DefaultPolicy()
	}
	return &OpenAIProvider{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, model: cfg.Model, httpClient: client, retry: cfg.Retry}
}

func (p *OpenAIProvider) GenerateText(ctx context.Context, messages []Message) (string, error) {
	body, err := p.marshalChatRequest(messages)
	if err != nil {
		return "", err
	}

	payload, err := p.doChatCompletion(ctx, body)
	if err != nil {
		return "", err
	}

	return decodeChatCompletion(payload)
}

func (p *OpenAIProvider) marshalChatRequest(messages []Message) ([]byte, error) {
	return json.Marshal(map[string]any{"model": p.model, "messages": messages})
}

func (p *OpenAIProvider) doChatCompletion(ctx context.Context, body []byte) (chatCompletionResponse, error) {
	var payload chatCompletionResponse
	err := p.retry.Do(ctx, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return retry.HTTPStatusError{StatusCode: resp.StatusCode}
		}

		return json.NewDecoder(resp.Body).Decode(&payload)
	})
	if err != nil {
		return chatCompletionResponse{}, err
	}
	return payload, nil
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func decodeChatCompletion(payload chatCompletionResponse) (string, error) {
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("openai response has no choices")
	}
	return payload.Choices[0].Message.Content, nil
}
