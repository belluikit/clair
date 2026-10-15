package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/belluikit/clair/kb"
	"github.com/belluikit/clair/parser"
	"github.com/fsnotify/fsnotify"
)

// Indexer manages full and incremental indexing of project source code.
type Indexer struct {
	kb      *kb.ProjectKB
	parser  parser.Parser
	roots   []string
	filter  *Filter
	watcher *Watcher
}

// Config holds indexer configuration.
type Config struct {
	// ExcludePatterns are glob patterns for files to skip. Nil uses defaults.
	ExcludePatterns []string

	// DebounceDuration is the debounce interval for file watch events.
	// Zero uses the default (500ms).
	DebounceDuration time.Duration
}

// New creates an Indexer for the given project KB, parser, and root directories.
func New(projectKB *kb.ProjectKB, p parser.Parser, roots []string, cfg *Config) *Indexer {
	var excludes []string
	if cfg != nil {
		excludes = cfg.ExcludePatterns
	}

	return &Indexer{
		kb:     projectKB,
		parser: p,
		roots:  roots,
		filter: NewFilter(excludes, p.Extensions()),
	}
}

// FullIndex walks all root directories and indexes every matching file.
// It clears the existing index first.
func (idx *Indexer) FullIndex() error {
	if err := idx.kb.Clear(); err != nil {
		return fmt.Errorf("clair: clear index: %w", err)
	}

	for _, root := range idx.roots {
		if err := idx.indexDir(root); err != nil {
			return err
		}
	}

	return nil
}

// Start begins watching for file changes and applying incremental updates.
func (idx *Indexer) Start() error {
	var debounce time.Duration
	w, err := NewWatcher(idx.filter, debounce)
	if err != nil {
		return fmt.Errorf("clair: create watcher: %w", err)
	}

	for _, root := range idx.roots {
		if err := w.AddRecursive(root); err != nil {
			w.Close()
			return fmt.Errorf("clair: watch %q: %w", root, err)
		}
	}

	idx.watcher = w

	go idx.watchLoop()
	return nil
}

// Stop stops watching for file changes.
func (idx *Indexer) Stop() error {
	if idx.watcher == nil {
		return nil
	}
	err := idx.watcher.Close()
	idx.watcher = nil
	return err
}

func (idx *Indexer) watchLoop() {
	for event := range idx.watcher.Events() {
		idx.handleFileChange(event.Path, event.Op)
	}
}

// handleFileChange processes a single file change event.
func (idx *Indexer) handleFileChange(path string, op fsnotify.Op) {
	if op.Has(fsnotify.Remove) || op.Has(fsnotify.Rename) {
		idx.kb.RemoveByFile(path)
		return
	}

	// Create or modify — re-index the file.
	idx.indexFile(path)
}

func (idx *Indexer) indexDir(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if !idx.filter.AcceptDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !idx.filter.Accept(path) {
			return nil
		}

		idx.indexFile(path)
		return nil
	})
}

func (idx *Indexer) indexFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	symbols, err := idx.parser.ParseFile(path, content)
	if err != nil || len(symbols) == 0 {
		return
	}

	// Set file path on all symbols.
	for i := range symbols {
		symbols[i].FilePath = path
	}

	// Remove old entries for this file, then add new ones.
	idx.kb.RemoveByFile(path)
	idx.kb.Index(symbols)
}
