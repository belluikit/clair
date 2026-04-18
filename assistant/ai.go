package assistant

import (
	"context"
	"errors"

	"github.com/trendvidia/clair/kb"
	"github.com/trendvidia/clair/llm"
	"github.com/trendvidia/clair/privacy"
)

// AIAssistant provides LLM-powered code assistance features.
type AIAssistant struct {
	kb           *kb.MergedKB
	provider     llm.Provider
	assembler    *llm.ContextAssembler
	conversation *llm.Conversation
	privacy      privacy.Policy
	systemPrompt string
}

// HasProvider returns true if an LLM provider is configured.
func (a *AIAssistant) HasProvider() bool {
	return a.provider != nil
}

// SetProvider configures the LLM provider at runtime.
func (a *AIAssistant) SetProvider(p llm.Provider) {
	a.provider = p
	caps := p.Capabilities()
	a.assembler = llm.NewContextAssembler(caps)
	a.conversation = llm.NewConversation(caps.MaxContextTokens / 4)
}

// Ask sends a query to the LLM with automatically retrieved KB context.
// The editor context is used to find relevant documentation.
func (a *AIAssistant) Ask(ctx context.Context, query string, editor EditorContext) (llm.ResponseStream, error) {
	if a.provider == nil {
		return nil, errors.New("clair: no LLM provider configured")
	}

	// Search KB for relevant docs.
	results, err := a.kb.Search(query, 10)
	if err != nil {
		return nil, err
	}

	// Add editor context to search if available.
	if editor.VisibleCode != "" {
		editorResults, err := a.kb.Search(editor.VisibleCode[:min(len(editor.VisibleCode), 200)], 5)
		if err == nil {
			results = append(results, editorResults...)
		}
	}

	// Apply privacy filtering.
	results = a.privacy.Filter(results)

	// Convert to LLM chunks.
	chunks := resultsToChunks(results)

	// Add user message to conversation.
	a.conversation.Add(llm.Message{Role: llm.RoleUser, Content: query})

	// Assemble the request within token budget.
	req := a.assembler.Assemble(a.systemPrompt, a.conversation.Messages(), chunks)

	// Stream the response.
	stream, err := a.provider.Stream(ctx, req)
	if err != nil {
		// Fall back to non-streaming if streaming isn't supported.
		if !a.provider.Capabilities().SupportsStreaming {
			return a.completeFallback(ctx, req)
		}
		return nil, err
	}

	return &trackedStream{
		inner:        stream,
		conversation: a.conversation,
	}, nil
}

// Complete sends a query and returns the full response (non-streaming).
func (a *AIAssistant) Complete(ctx context.Context, query string, editor EditorContext) (*llm.Response, error) {
	if a.provider == nil {
		return nil, errors.New("clair: no LLM provider configured")
	}

	results, err := a.kb.Search(query, 10)
	if err != nil {
		return nil, err
	}

	results = a.privacy.Filter(results)
	chunks := resultsToChunks(results)

	a.conversation.Add(llm.Message{Role: llm.RoleUser, Content: query})

	req := a.assembler.Assemble(a.systemPrompt, a.conversation.Messages(), chunks)

	resp, err := a.provider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	a.conversation.Add(llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
	return resp, nil
}

// ClearConversation resets the conversation history.
func (a *AIAssistant) ClearConversation() {
	if a.conversation != nil {
		a.conversation.Clear()
	}
}

func (a *AIAssistant) completeFallback(ctx context.Context, req llm.Request) (llm.ResponseStream, error) {
	resp, err := a.provider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	a.conversation.Add(llm.Message{Role: llm.RoleAssistant, Content: resp.Content})

	return &singleResponseStream{content: resp.Content}, nil
}

func resultsToChunks(results []kb.SearchResult) []llm.Chunk {
	seen := make(map[string]bool)
	var chunks []llm.Chunk
	for _, r := range results {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true

		content := r.Signature
		if r.Doc != "" {
			content += "\n" + r.Doc
		}
		if r.Example != "" {
			content += "\n\nExample:\n" + r.Example
		}

		chunks = append(chunks, llm.Chunk{
			Source:  r.Tier.String(),
			Title:   r.ID,
			Content: content,
		})
	}
	return chunks
}

// trackedStream wraps a ResponseStream to record the assistant's response
// in the conversation history.
type trackedStream struct {
	inner        llm.ResponseStream
	conversation *llm.Conversation
	accumulated  string
}

func (s *trackedStream) Next() (llm.StreamEvent, error) {
	event, err := s.inner.Next()
	if event.Type == llm.EventTextDelta {
		s.accumulated += event.Text
	}
	if event.Type == llm.EventDone {
		s.conversation.Add(llm.Message{Role: llm.RoleAssistant, Content: s.accumulated})
	}
	return event, err
}

func (s *trackedStream) Close() error {
	return s.inner.Close()
}

// singleResponseStream wraps a complete response as a stream.
type singleResponseStream struct {
	content  string
	consumed bool
}

func (s *singleResponseStream) Next() (llm.StreamEvent, error) {
	if s.consumed {
		return llm.StreamEvent{Type: llm.EventDone}, nil
	}
	s.consumed = true
	return llm.StreamEvent{Type: llm.EventTextDelta, Text: s.content}, nil
}

func (s *singleResponseStream) Close() error { return nil }
