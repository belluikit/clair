package kb

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/trendvidia/clair/parser"
)

// ProjectKB is a read-write knowledge base for the user's project.
// It auto-indexes project source code and supports incremental updates.
type ProjectKB struct {
	index bleve.Index
}

// OpenProject opens an existing project index, or creates a new one at the given path.
func OpenProject(indexPath string) (*ProjectKB, error) {
	idx, err := bleve.Open(indexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		idx, err = bleve.New(indexPath, NewIndexMapping())
		if err != nil {
			return nil, fmt.Errorf("clair: create project index %q: %w", indexPath, err)
		}
		return &ProjectKB{index: idx}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clair: open project index %q: %w", indexPath, err)
	}
	return &ProjectKB{index: idx}, nil
}

// Close closes the underlying Bleve index.
func (kb *ProjectKB) Close() error {
	return kb.index.Close()
}

// Index adds or updates symbols in the knowledge base.
func (kb *ProjectKB) Index(symbols []parser.Symbol) error {
	batch := kb.index.NewBatch()
	for _, sym := range symbols {
		doc := DocumentFromSymbol(sym, TierProject)
		if err := batch.Index(doc.ID, ToIndexDoc(doc)); err != nil {
			return fmt.Errorf("clair: index %q: %w", doc.ID, err)
		}
	}
	return kb.index.Batch(batch)
}

// Remove deletes a document by ID.
func (kb *ProjectKB) Remove(id string) error {
	return kb.index.Delete(id)
}

// RemoveByFile removes all documents associated with a file path.
func (kb *ProjectKB) RemoveByFile(filePath string) error {
	q := bleve.NewTermQuery(filePath)
	q.SetField("filePath")
	req := bleve.NewSearchRequestOptions(q, 1000, 0, false)

	res, err := kb.index.Search(req)
	if err != nil {
		return fmt.Errorf("clair: search by file %q: %w", filePath, err)
	}

	if len(res.Hits) == 0 {
		return nil
	}

	batch := kb.index.NewBatch()
	for _, hit := range res.Hits {
		batch.Delete(hit.ID)
	}
	return kb.index.Batch(batch)
}

// Clear removes all documents from the index.
func (kb *ProjectKB) Clear() error {
	// Bleve doesn't have a bulk-clear; we search all and delete.
	q := bleve.NewMatchAllQuery()
	req := bleve.NewSearchRequestOptions(q, 10000, 0, false)

	res, err := kb.index.Search(req)
	if err != nil {
		return fmt.Errorf("clair: clear: %w", err)
	}

	if len(res.Hits) == 0 {
		return nil
	}

	batch := kb.index.NewBatch()
	for _, hit := range res.Hits {
		batch.Delete(hit.ID)
	}
	return kb.index.Batch(batch)
}

// LookupSymbol finds a document by exact ID.
func (kb *ProjectKB) LookupSymbol(name string) (*Document, error) {
	doc, err := lookupByID(kb.index, name)
	if err != nil {
		return nil, err
	}
	if doc != nil {
		doc.Tier = TierProject
	}
	return doc, nil
}

// Search performs a full-text search and returns ranked results.
func (kb *ProjectKB) Search(query string, limit int) ([]SearchResult, error) {
	return searchIndex(kb.index, query, limit, TierProject)
}

// DocCount returns the number of documents in the index.
func (kb *ProjectKB) DocCount() (uint64, error) {
	return kb.index.DocCount()
}
