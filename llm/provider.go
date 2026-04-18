// Package llm provides an abstracted interface for LLM providers.
package llm

import "context"

// Provider is the interface that LLM backends must implement.
type Provider interface {
	// Complete sends a request and returns the full response.
	Complete(ctx context.Context, req Request) (*Response, error)

	// Stream sends a request and returns a streaming response.
	Stream(ctx context.Context, req Request) (ResponseStream, error)

	// Capabilities returns what this provider supports.
	Capabilities() Capabilities

	// Close releases provider resources.
	Close() error
}

// Request is a completion request sent to the LLM.
type Request struct {
	// System is the system prompt (framework reference, instructions).
	System string

	// Messages is the conversation history.
	Messages []Message

	// Context contains KB-retrieved documents injected as additional context.
	Context []Chunk

	// MaxTokens limits the response length. 0 means provider default.
	MaxTokens int
}

// Message is a single turn in the conversation.
type Message struct {
	Role    Role
	Content string
}

// Role identifies the message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Chunk is a piece of KB-retrieved context included in the request.
type Chunk struct {
	// Source identifies where this chunk came from (e.g. "framework", "project").
	Source string

	// Title is a short label (e.g. "widget.NewList").
	Title string

	// Content is the chunk text (signature, doc, example).
	Content string
}

// Response is the full result from a completion request.
type Response struct {
	Content    string
	TokensUsed TokenUsage
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens int
	OutputTokens int
	TotalTokens  int
}

// ResponseStream provides streaming access to a completion response.
type ResponseStream interface {
	// Next returns the next event. Returns io.EOF when done.
	Next() (StreamEvent, error)

	// Close releases stream resources.
	Close() error
}

// StreamEvent is a single event in a streaming response.
type StreamEvent struct {
	// Type indicates the event kind.
	Type StreamEventType

	// Text is the content delta (only for EventTextDelta).
	Text string

	// Error is set when Type is EventError.
	Error error
}

// StreamEventType classifies stream events.
type StreamEventType int

const (
	EventTextDelta StreamEventType = iota
	EventDone
	EventError
)

// Capabilities describes what a provider supports.
type Capabilities struct {
	// MaxContextTokens is the maximum number of tokens in the context window.
	MaxContextTokens int

	// SupportsStreaming indicates if Stream() is supported.
	SupportsStreaming bool

	// SupportsSystem indicates if the provider handles system prompts natively.
	SupportsSystem bool
}
