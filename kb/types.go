// Package kb provides a two-tier knowledge base for framework and project documentation.
package kb

import "github.com/belluikit/clair/parser"

// Tier identifies which knowledge base layer a document belongs to.
type Tier int

const (
	// TierFramework is the static, shipped knowledge base for the framework.
	TierFramework Tier = iota
	// TierProject is the dynamic, auto-indexed knowledge base for the user's project.
	TierProject
)

func (t Tier) String() string {
	if t == TierFramework {
		return "framework"
	}
	return "project"
}

// Document represents an indexed symbol in the knowledge base.
type Document struct {
	// ID uniquely identifies this document (e.g. "widget.NewList").
	ID string

	// Tier indicates which KB layer this document belongs to.
	Tier Tier

	// Symbol is the short name (e.g. "NewList").
	Symbol string

	// Package is the package name (e.g. "widget").
	Package string

	// Kind is the symbol classification.
	Kind parser.SymbolKind

	// Signature is the full type or function signature.
	Signature string

	// Doc is the documentation text.
	Doc string

	// Example contains usage example code.
	Example string

	// FilePath is the source file where the symbol is defined.
	FilePath string

	// Line is the line number of the declaration.
	Line int

	// References lists framework types used by this symbol.
	References []string

	// Metadata holds additional key-value data.
	Metadata map[string]string
}

// DocumentFromSymbol creates a Document from a parsed Symbol.
func DocumentFromSymbol(sym parser.Symbol, tier Tier) Document {
	return Document{
		ID:         sym.ID(),
		Tier:       tier,
		Symbol:     sym.Name,
		Package:    sym.Package,
		Kind:       sym.Kind,
		Signature:  sym.Signature,
		Doc:        sym.Doc,
		FilePath:   sym.FilePath,
		Line:       sym.Line,
		References: sym.References,
	}
}

// SearchResult is a Document with a relevance score and match context.
type SearchResult struct {
	Document

	// Score is the relevance score from the search engine.
	Score float64

	// Snippet is a highlighted excerpt showing the match context.
	Snippet string
}

// Config holds configuration for the knowledge base.
type Config struct {
	// FrameworkIndexPath is the path to the pre-built framework Bleve index.
	FrameworkIndexPath string

	// ProjectIndexPath is the path where the project index will be created.
	ProjectIndexPath string

	// ProjectRoots are the directories to scan for project source code.
	ProjectRoots []string

	// ExcludePatterns are glob patterns for files to skip during indexing.
	ExcludePatterns []string

	// IncludeKinds limits indexing to specific symbol kinds. Nil means all kinds.
	IncludeKinds []parser.SymbolKind
}
