// Package assistant provides the orchestration layer combining
// built-in KB search with optional AI-powered assistance.
package assistant

import (
	"github.com/belluikit/clair/kb"
	"github.com/belluikit/clair/llm"
	"github.com/belluikit/clair/privacy"
)

// EditorContext provides information about the developer's current editing state.
type EditorContext struct {
	// CurrentFile is the path of the file being edited.
	CurrentFile string

	// VisibleCode is the code currently visible in the editor viewport.
	VisibleCode string

	// CursorLine is the line number where the cursor is positioned.
	CursorLine int

	// Language is the programming language of the current file.
	Language string
}

// Assistant is the main entry point combining built-in and AI-powered features.
type Assistant struct {
	Builtin *BuiltinAssistant
	AI      *AIAssistant
}

// Option configures the Assistant.
type Option func(*Assistant)

// WithProvider sets the LLM provider for AI features.
func WithProvider(p llm.Provider) Option {
	return func(a *Assistant) {
		a.AI.provider = p
		caps := p.Capabilities()
		a.AI.assembler = llm.NewContextAssembler(caps)
		a.AI.conversation = llm.NewConversation(caps.MaxContextTokens / 4)
	}
}

// WithPrivacy sets the privacy policy for filtering content sent to the LLM.
func WithPrivacy(p privacy.Policy) Option {
	return func(a *Assistant) {
		a.AI.privacy = p
	}
}

// WithSystemPrompt sets the system prompt for AI interactions.
func WithSystemPrompt(prompt string) Option {
	return func(a *Assistant) {
		a.AI.systemPrompt = prompt
	}
}

// New creates an Assistant with the given knowledge bases and options.
// frameworkKB can be nil if no framework index is available.
// projectKB can be nil if project indexing is not needed.
func New(frameworkKB *kb.FrameworkKB, projectKB *kb.ProjectKB, opts ...Option) *Assistant {
	merged := kb.NewMergedKB(frameworkKB, projectKB)

	a := &Assistant{
		Builtin: &BuiltinAssistant{kb: merged},
		AI: &AIAssistant{
			kb:      merged,
			privacy: privacy.DefaultPolicy(),
		},
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// HasAI returns true if an LLM provider is configured.
func (a *Assistant) HasAI() bool {
	return a.AI.HasProvider()
}

// Close releases all resources.
func (a *Assistant) Close() error {
	if a.AI.provider != nil {
		a.AI.provider.Close()
	}
	return a.Builtin.kb.Close()
}
