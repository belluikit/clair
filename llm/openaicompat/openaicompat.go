// Package openaicompat provides an OpenAI-compatible LLM provider.
// It works with OpenAI, Azure OpenAI, LM Studio, vLLM, llama.cpp server,
// and any other service implementing the OpenAI chat completions API.
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/belluikit/clair/llm"
)

// Provider implements llm.Provider for OpenAI-compatible APIs.
type Provider struct {
	baseURL    string
	apiKey     string
	model      string
	maxContext int
	client     *http.Client
}

// Config holds provider configuration.
type Config struct {
	// BaseURL is the API base URL (e.g. "https://api.openai.com/v1").
	BaseURL string

	// APIKey is the API key (optional for local models).
	APIKey string

	// Model is the model name (e.g. "gpt-4o", "llama3").
	Model string

	// MaxContextTokens overrides the default context window size.
	MaxContextTokens int
}

// New creates an OpenAI-compatible provider.
func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if cfg.MaxContextTokens == 0 {
		cfg.MaxContextTokens = 128000
	}

	return &Provider{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxContext: cfg.MaxContextTokens,
		client:     &http.Client{},
	}
}

func (p *Provider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		MaxContextTokens:  p.maxContext,
		SupportsStreaming: true,
		SupportsSystem:    true,
	}
}

func (p *Provider) Close() error { return nil }

// chatRequest is the OpenAI chat completion request body.
type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Stream    bool          `json:"stream,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *Provider) buildMessages(req llm.Request) []chatMessage {
	var msgs []chatMessage

	// System prompt.
	systemContent := req.System
	if len(req.Context) > 0 {
		systemContent += "\n\n" + llm.FormatContext(req.Context)
	}
	if systemContent != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: systemContent})
	}

	// Conversation history.
	for _, m := range req.Messages {
		msgs = append(msgs, chatMessage{Role: string(m.Role), Content: m.Content})
	}

	return msgs
}

func (p *Provider) doRequest(ctx context.Context, body chatRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	return p.client.Do(httpReq)
}

func (p *Provider) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	body := chatRequest{
		Model:    p.model,
		Messages: p.buildMessages(req),
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	}

	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("openaicompat: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openaicompat: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("openaicompat: decode: %w", err)
	}

	content := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
	}

	return &llm.Response{
		Content: content,
		TokensUsed: llm.TokenUsage{
			PromptTokens: chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
		},
	}, nil
}

func (p *Provider) Stream(ctx context.Context, req llm.Request) (llm.ResponseStream, error) {
	body := chatRequest{
		Model:    p.model,
		Messages: p.buildMessages(req),
		Stream:   true,
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	}

	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("openaicompat: request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openaicompat: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &sseStream{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// sseStream reads Server-Sent Events from the HTTP response.
type sseStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	done    bool
}

func (s *sseStream) Next() (llm.StreamEvent, error) {
	if s.done {
		return llm.StreamEvent{Type: llm.EventDone}, io.EOF
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			s.done = true
			return llm.StreamEvent{Type: llm.EventDone}, nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			return llm.StreamEvent{
				Type: llm.EventTextDelta,
				Text: choice.Delta.Content,
			}, nil
		}

		if choice.FinishReason != nil {
			s.done = true
			return llm.StreamEvent{Type: llm.EventDone}, nil
		}
	}

	if err := s.scanner.Err(); err != nil {
		return llm.StreamEvent{Type: llm.EventError, Error: err}, err
	}

	s.done = true
	return llm.StreamEvent{Type: llm.EventDone}, io.EOF
}

func (s *sseStream) Close() error {
	return s.body.Close()
}
