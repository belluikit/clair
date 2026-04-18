// Package ollama provides an LLM provider for locally-running Ollama instances.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/trendvidia/clair/llm"
)

// Provider implements llm.Provider for Ollama's API.
type Provider struct {
	baseURL    string
	model      string
	maxContext int
	client     *http.Client
}

// Config holds Ollama provider configuration.
type Config struct {
	// BaseURL is the Ollama server URL (default "http://localhost:11434").
	BaseURL string

	// Model is the model name (e.g. "llama3", "codellama").
	Model string

	// MaxContextTokens overrides the default context window size.
	MaxContextTokens int
}

// New creates an Ollama provider.
func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if cfg.MaxContextTokens == 0 {
		cfg.MaxContextTokens = 8192
	}

	return &Provider{
		baseURL:    cfg.BaseURL,
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

// chatRequest is the Ollama chat API request body.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (p *Provider) buildMessages(req llm.Request) []chatMessage {
	var msgs []chatMessage

	systemContent := req.System
	if len(req.Context) > 0 {
		systemContent += "\n\n" + llm.FormatContext(req.Context)
	}
	if systemContent != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: systemContent})
	}

	for _, m := range req.Messages {
		msgs = append(msgs, chatMessage{Role: string(m.Role), Content: m.Content})
	}

	return msgs
}

func (p *Provider) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	body := chatRequest{
		Model:    p.model,
		Messages: p.buildMessages(req),
		Stream:   false,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("ollama: decode: %w", err)
	}

	return &llm.Response{
		Content: chatResp.Message.Content,
	}, nil
}

func (p *Provider) Stream(ctx context.Context, req llm.Request) (llm.ResponseStream, error) {
	body := chatRequest{
		Model:    p.model,
		Messages: p.buildMessages(req),
		Stream:   true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &ndjsonStream{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// ndjsonStream reads newline-delimited JSON from Ollama's streaming response.
type ndjsonStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	done    bool
}

func (s *ndjsonStream) Next() (llm.StreamEvent, error) {
	if s.done {
		return llm.StreamEvent{Type: llm.EventDone}, io.EOF
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			continue
		}

		var resp chatResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}

		if resp.Done {
			s.done = true
			if resp.Message.Content != "" {
				return llm.StreamEvent{
					Type: llm.EventTextDelta,
					Text: resp.Message.Content,
				}, nil
			}
			return llm.StreamEvent{Type: llm.EventDone}, nil
		}

		if resp.Message.Content != "" {
			return llm.StreamEvent{
				Type: llm.EventTextDelta,
				Text: resp.Message.Content,
			}, nil
		}
	}

	if err := s.scanner.Err(); err != nil {
		return llm.StreamEvent{Type: llm.EventError, Error: err}, err
	}

	s.done = true
	return llm.StreamEvent{Type: llm.EventDone}, io.EOF
}

func (s *ndjsonStream) Close() error {
	return s.body.Close()
}
