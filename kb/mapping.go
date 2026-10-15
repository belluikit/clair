package kb

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// NewIndexMapping creates a Bleve index mapping optimized for code symbol search.
func NewIndexMapping() mapping.IndexMapping {
	// Keyword field — indexed but not analyzed (exact match).
	keywordField := func() *mapping.FieldMapping {
		f := bleve.NewKeywordFieldMapping()
		f.Store = true
		return f
	}

	// Text field — analyzed for full-text search.
	textField := func() *mapping.FieldMapping {
		f := bleve.NewTextFieldMapping()
		f.Store = true
		return f
	}

	// Numeric field.
	numericField := func() *mapping.FieldMapping {
		f := bleve.NewNumericFieldMapping()
		f.Store = true
		f.Index = false
		return f
	}

	doc := bleve.NewDocumentMapping()
	doc.AddFieldMappingsAt("id", keywordField())
	doc.AddFieldMappingsAt("tier", numericField())
	doc.AddFieldMappingsAt("symbol", textField())        // symbol names
	doc.AddFieldMappingsAt("package", textField())       // package-qualified searches
	doc.AddFieldMappingsAt("kind", keywordField())       // exact match: "func", "type", etc.
	doc.AddFieldMappingsAt("signature", textField())     // function/type signatures
	doc.AddFieldMappingsAt("doc", textField())           // documentation text
	doc.AddFieldMappingsAt("example", textField())       // usage examples
	doc.AddFieldMappingsAt("filePath", keywordField())   // exact match
	doc.AddFieldMappingsAt("line", numericField())       // not searched
	doc.AddFieldMappingsAt("references", keywordField()) // exact match on framework refs

	im := bleve.NewIndexMapping()
	im.DefaultMapping = doc

	return im
}

// indexDoc is the structure written to the Bleve index.
// Field names match the mapping keys above.
type indexDoc struct {
	ID         string   `json:"id"`
	Tier       int      `json:"tier"`
	Symbol     string   `json:"symbol"`
	Package    string   `json:"package"`
	Kind       string   `json:"kind"`
	Signature  string   `json:"signature"`
	Doc        string   `json:"doc"`
	Example    string   `json:"example"`
	FilePath   string   `json:"filePath"`
	Line       int      `json:"line"`
	References []string `json:"references"`
}

// ToIndexDoc converts a Document to the flat index format.
func ToIndexDoc(d Document) indexDoc {
	return indexDoc{
		ID:         d.ID,
		Tier:       int(d.Tier),
		Symbol:     d.Symbol,
		Package:    d.Package,
		Kind:       d.Kind.String(),
		Signature:  d.Signature,
		Doc:        d.Doc,
		Example:    d.Example,
		FilePath:   d.FilePath,
		Line:       d.Line,
		References: d.References,
	}
}
