// Package builder provides tools for constructing framework knowledge base indexes.
package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
	"github.com/trendvidia/clair/kb"
	"github.com/trendvidia/clair/parser"
)

// Builder constructs a framework Bleve index from source code and manual entries.
type Builder struct {
	docs []kb.Document
}

// New creates a new Builder.
func New() *Builder {
	return &Builder{}
}

// Add adds a manually constructed document to the index.
func (b *Builder) Add(doc kb.Document) {
	doc.Tier = kb.TierFramework
	b.docs = append(b.docs, doc)
}

// AddFromParser walks a source directory and extracts symbols using the given parser.
func (b *Builder) AddFromParser(p parser.Parser, sourceDir string) error {
	exts := make(map[string]bool, len(p.Extensions()))
	for _, ext := range p.Extensions() {
		exts[ext] = true
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors, don't block the build
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" || base == "testdata" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !exts[ext] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		symbols, err := p.ParseFile(path, content)
		if err != nil {
			return nil // skip unparseable files
		}

		for _, sym := range symbols {
			b.docs = append(b.docs, kb.DocumentFromSymbol(sym, kb.TierFramework))
		}

		return nil
	})
}

// Build writes the accumulated documents to a new Bleve index at outputPath.
// The outputPath must not already exist.
func (b *Builder) Build(outputPath string) error {
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("clair: index path %q already exists", outputPath)
	}

	idx, err := bleve.New(outputPath, kb.NewIndexMapping())
	if err != nil {
		return fmt.Errorf("clair: create index: %w", err)
	}
	defer idx.Close()

	batch := idx.NewBatch()
	for i, doc := range b.docs {
		if err := batch.Index(doc.ID, kb.ToIndexDoc(doc)); err != nil {
			return fmt.Errorf("clair: index doc %q: %w", doc.ID, err)
		}
		// Flush batch every 1000 docs.
		if (i+1)%1000 == 0 {
			if err := idx.Batch(batch); err != nil {
				return fmt.Errorf("clair: batch write: %w", err)
			}
			batch = idx.NewBatch()
		}
	}

	// Flush remaining.
	if batch.Size() > 0 {
		if err := idx.Batch(batch); err != nil {
			return fmt.Errorf("clair: batch write: %w", err)
		}
	}

	return nil
}

// DocCount returns the number of documents added so far.
func (b *Builder) DocCount() int {
	return len(b.docs)
}
