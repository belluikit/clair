package llm

import (
	"fmt"
	"strings"
)

// ContextAssembler builds LLM requests with token-aware context budgeting.
type ContextAssembler struct {
	// MaxTokens is the provider's context window size.
	MaxTokens int

	// ResponseMargin is tokens reserved for the response (default 1024).
	ResponseMargin int
}

// NewContextAssembler creates a ContextAssembler for the given capabilities.
func NewContextAssembler(caps Capabilities) *ContextAssembler {
	return &ContextAssembler{
		MaxTokens:      caps.MaxContextTokens,
		ResponseMargin: 1024,
	}
}

// Assemble builds a Request from the system prompt, conversation history,
// and KB-retrieved chunks, fitting everything within the token budget.
func (ca *ContextAssembler) Assemble(system string, history []Message, chunks []Chunk) Request {
	budget := ca.MaxTokens - ca.ResponseMargin
	if budget <= 0 {
		budget = ca.MaxTokens
	}

	// System prompt is always included.
	used := EstimateTokens(system)
	budget -= used

	// History: include recent messages first, drop oldest if over budget.
	var messages []Message
	historyTokens := 0
	for i := len(history) - 1; i >= 0 && budget > 0; i-- {
		msgTokens := EstimateTokens(history[i].Content) + 4 // overhead for role
		if historyTokens+msgTokens > budget/2 {
			break // reserve half the remaining budget for context
		}
		messages = append([]Message{history[i]}, messages...)
		historyTokens += msgTokens
	}
	budget -= historyTokens

	// KB context: include highest-priority chunks that fit.
	var context []Chunk
	for _, chunk := range chunks {
		chunkTokens := EstimateTokens(chunk.Content) + EstimateTokens(chunk.Title) + 10
		if chunkTokens > budget {
			continue
		}
		context = append(context, chunk)
		budget -= chunkTokens
	}

	return Request{
		System:   system,
		Messages: messages,
		Context:  context,
	}
}

// FormatContext renders KB chunks as a text block suitable for injection
// into the system prompt or as a preamble message.
func FormatContext(chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Relevant Documentation\n\n")

	currentSource := ""
	for _, chunk := range chunks {
		if chunk.Source != currentSource {
			currentSource = chunk.Source
			fmt.Fprintf(&b, "### %s\n\n", strings.Title(currentSource))
		}
		fmt.Fprintf(&b, "**%s**\n%s\n\n", chunk.Title, chunk.Content)
	}

	return b.String()
}

// EstimateTokens provides a rough token count estimate.
// Uses the heuristic of ~4 characters per token with a 10% safety margin.
func EstimateTokens(text string) int {
	chars := len(text)
	tokens := (chars + 3) / 4 // ceil division
	return tokens + tokens/10 // +10% margin
}
