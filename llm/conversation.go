package llm

import "sync"

// Conversation manages a rolling message history with a token budget.
type Conversation struct {
	mu        sync.Mutex
	messages  []Message
	maxTokens int
}

// NewConversation creates a Conversation with the given token budget for history.
func NewConversation(maxTokens int) *Conversation {
	return &Conversation{
		maxTokens: maxTokens,
	}
}

// Add appends a message and trims oldest messages if over budget.
func (c *Conversation) Add(msg Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, msg)
	c.trim()
}

// Messages returns a copy of the current history.
func (c *Conversation) Messages() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]Message, len(c.messages))
	copy(out, c.messages)
	return out
}

// Clear removes all messages.
func (c *Conversation) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = nil
}

// trim removes oldest messages to stay within the token budget.
func (c *Conversation) trim() {
	for c.totalTokens() > c.maxTokens && len(c.messages) > 1 {
		c.messages = c.messages[1:]
	}
}

func (c *Conversation) totalTokens() int {
	total := 0
	for _, msg := range c.messages {
		total += EstimateTokens(msg.Content) + 4
	}
	return total
}
