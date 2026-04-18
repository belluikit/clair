// Package indexer manages automatic project source code indexing.
package indexer

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Filter determines which files should be indexed based on include/exclude patterns.
type Filter struct {
	excludePatterns []string
	includeExts     map[string]bool
}

// DefaultExcludePatterns are the default glob patterns for files to skip.
var DefaultExcludePatterns = []string{
	"**/vendor/**",
	"**/.git/**",
	"**/testdata/**",
	"**/node_modules/**",
	"**/*_test.go",
	"**/.myide/**",
}

// NewFilter creates a Filter with the given exclusions and file extensions.
// If excludePatterns is nil, DefaultExcludePatterns is used.
func NewFilter(excludePatterns []string, extensions []string) *Filter {
	if excludePatterns == nil {
		excludePatterns = DefaultExcludePatterns
	}

	exts := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts[ext] = true
	}

	return &Filter{
		excludePatterns: excludePatterns,
		includeExts:     exts,
	}
}

// Accept returns true if the file should be indexed.
func (f *Filter) Accept(path string) bool {
	// Check extension.
	if len(f.includeExts) > 0 {
		ext := filepath.Ext(path)
		if !f.includeExts[ext] {
			return false
		}
	}

	// Check exclude patterns.
	for _, pattern := range f.excludePatterns {
		if matched, _ := doublestar.Match(pattern, path); matched {
			return false
		}
	}

	return true
}

// AcceptDir returns true if a directory should be traversed.
func (f *Filter) AcceptDir(path string) bool {
	base := filepath.Base(path)
	// Quick reject common directories.
	switch base {
	case ".git", "vendor", "node_modules", "testdata", ".myide":
		return false
	}
	return true
}
