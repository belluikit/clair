// Package anthropic provides an LLM provider for the Anthropic Messages API.
package anthropic

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

// Provider implements llm.Provider for the Anthropic API.
type Provider struct {
	apiKey     string
	model      string
	maxContext int
	client     *http.Client
}

// Config holds Anthropic provider configuration.
type Config struct {
	// APIKey is the Anthropic API key.
	APIKey string

	// Model is the model ID (default "claude-sonnet-4-20250514").
	Model string

	// MaxContextTokens overrides the default context window size.
	MaxContextTokens int
}

// New creates an Anthropic provider.
func New(cfg Config) *Provider {
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.MaxContextTokens == 0 {
		cfg.MaxContextTokens = 200000
	}

	return &Provider{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxContext: cfg.MaxContextTokens,
		client:     &http.Client{},
	}
}

func (p *Provider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		MaxContextTokens: p.maxContext,
		SupportsStreaming: true,
		SupportsSystem:   true,
	}
}

func (p *Provider) Close() error { return nil }

type messagesRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []msgContent  `json:"messages"`
	Stream    bool          `json:"stream,omitempty"`
}

type msgContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type streamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

func (p *Provider) buildRequest(req llm.Request, stream bool) messagesRequest {
	system := req.System
	if len(req.Context) > 0 {
		system += "\n\n" + llm.FormatContext(req.Context)
	}

	var msgs []msgContent
	for _, m := range req.Messages {
		msgs = append(msgs, msgContent{Role: string(m.Role), Content: m.Content})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return messagesRequest{
		Model:     p.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  msgs,
		Stream:    stream,
	}
}

func (p *Provider) doHTTP(ctx context.Context, body messagesRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	return p.client.Do(httpReq)
}

func (p *Provider) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	body := p.buildRequest(req, false)

	resp, err := p.doHTTP(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var msgResp messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode: %w", err)
	}

	content := ""
	if len(msgResp.Content) > 0 {
		content = msgResp.Content[0].Text
	}

	return &llm.Response{
		Content: content,
		TokensUsed: llm.TokenUsage{
			PromptTokens: msgResp.Usage.InputTokens,
			OutputTokens: msgResp.Usage.OutputTokens,
			TotalTokens:  msgResp.Usage.InputTokens + msgResp.Usage.OutputTokens,
		},
	}, nil
}

func (p *Provider) Stream(ctx context.Context, req llm.Request) (llm.ResponseStream, error) {
	body := p.buildRequest(req, true)

	resp, err := p.doHTTP(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &sseStream{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

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

		var event streamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Text != "" {
				return llm.StreamEvent{
					Type: llm.EventTextDelta,
					Text: event.Delta.Text,
				}, nil
			}
		case "message_stop":
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
