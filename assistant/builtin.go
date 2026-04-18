package assistant

import "github.com/trendvidia/clair/kb"

// BuiltinAssistant provides non-AI code intelligence features.
// Always works offline with no external dependencies.
type BuiltinAssistant struct {
	kb *kb.MergedKB
}

// LookupSymbol finds a document by exact name (e.g. "widget.NewList").
func (b *BuiltinAssistant) LookupSymbol(name string) (*kb.Document, error) {
	return b.kb.LookupSymbol(name)
}

// Search performs a keyword search across both framework and project KBs.
func (b *BuiltinAssistant) Search(query string, limit int) ([]kb.SearchResult, error) {
	return b.kb.Search(query, limit)
}

// RelatedTo finds symbols related to the code at the given file and line.
// It extracts identifiers from the surrounding code context and searches for them.
func (b *BuiltinAssistant) RelatedTo(ctx EditorContext, limit int) ([]kb.SearchResult, error) {
	// Use visible code as the search query — Bleve will extract terms.
	query := ctx.VisibleCode
	if len(query) > 500 {
		query = query[:500] // limit query size
	}
	return b.kb.Search(query, limit)
}
