package kb

import (
	"fmt"

	"github.com/belluikit/clair/parser"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/highlight/highlighter/ansi"
)

// FrameworkKB is a read-only knowledge base for a framework's API.
// It wraps a pre-built Bleve index shipped with the IDE.
type FrameworkKB struct {
	index bleve.Index
}

// OpenFramework opens a pre-built framework Bleve index at the given path.
func OpenFramework(indexPath string) (*FrameworkKB, error) {
	idx, err := bleve.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("clair: open framework index %q: %w", indexPath, err)
	}
	return &FrameworkKB{index: idx}, nil
}

// Close closes the underlying Bleve index.
func (kb *FrameworkKB) Close() error {
	return kb.index.Close()
}

// LookupSymbol finds a document by exact ID (e.g. "widget.NewList").
func (kb *FrameworkKB) LookupSymbol(name string) (*Document, error) {
	doc, err := lookupByID(kb.index, name)
	if err != nil {
		return nil, err
	}
	if doc != nil {
		doc.Tier = TierFramework
	}
	return doc, nil
}

// Search performs a full-text search and returns ranked results.
func (kb *FrameworkKB) Search(query string, limit int) ([]SearchResult, error) {
	return searchIndex(kb.index, query, limit, TierFramework)
}

// searchIndex is shared search logic for both framework and project KBs.
func searchIndex(idx bleve.Index, query string, limit int, tier Tier) ([]SearchResult, error) {
	q := bleve.NewMatchQuery(query)
	req := bleve.NewSearchRequestOptions(q, limit, 0, false)
	req.Fields = []string{"*"}
	req.Highlight = bleve.NewHighlightWithStyle(ansi.Name)

	res, err := idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("clair: search: %w", err)
	}

	results := make([]SearchResult, 0, len(res.Hits))
	for _, hit := range res.Hits {
		doc := documentFromFields(hit.Fields, tier)
		sr := SearchResult{
			Document: doc,
			Score:    hit.Score,
		}
		// Use highlighted fragment from doc field if available.
		if frags, ok := hit.Fragments["doc"]; ok && len(frags) > 0 {
			sr.Snippet = frags[0]
		}
		results = append(results, sr)
	}

	return results, nil
}

// lookupByID finds a single document by its ID field.
func lookupByID(idx bleve.Index, id string) (*Document, error) {
	q := bleve.NewTermQuery(id)
	q.SetField("id")
	req := bleve.NewSearchRequestOptions(q, 1, 0, false)
	req.Fields = []string{"*"}

	res, err := idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("clair: lookup %q: %w", id, err)
	}
	if len(res.Hits) == 0 {
		return nil, nil
	}

	doc := documentFromFields(res.Hits[0].Fields, TierFramework)
	return &doc, nil
}

// documentFromFields reconstructs a Document from Bleve stored fields.
func documentFromFields(fields map[string]interface{}, tier Tier) Document {
	d := Document{Tier: tier}

	if v, ok := fields["id"].(string); ok {
		d.ID = v
	}
	if v, ok := fields["symbol"].(string); ok {
		d.Symbol = v
	}
	if v, ok := fields["package"].(string); ok {
		d.Package = v
	}
	if v, ok := fields["kind"].(string); ok {
		d.Kind = parseSymbolKind(v)
	}
	if v, ok := fields["signature"].(string); ok {
		d.Signature = v
	}
	if v, ok := fields["doc"].(string); ok {
		d.Doc = v
	}
	if v, ok := fields["example"].(string); ok {
		d.Example = v
	}
	if v, ok := fields["filePath"].(string); ok {
		d.FilePath = v
	}
	if v, ok := fields["line"].(float64); ok {
		d.Line = int(v)
	}
	// References come back as individual string values from bleve.
	if v, ok := fields["references"].(string); ok && v != "" {
		d.References = []string{v}
	}
	if v, ok := fields["references"].([]interface{}); ok {
		for _, r := range v {
			if s, ok := r.(string); ok {
				d.References = append(d.References, s)
			}
		}
	}

	return d
}

// parseSymbolKind converts a string back to SymbolKind.
func parseSymbolKind(s string) parser.SymbolKind {
	switch s {
	case "type":
		return parser.KindType
	case "interface":
		return parser.KindInterface
	case "func":
		return parser.KindFunc
	case "method":
		return parser.KindMethod
	case "const":
		return parser.KindConst
	case "var":
		return parser.KindVar
	default:
		return parser.KindType
	}
}
