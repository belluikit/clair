// Package privacy provides data flow controls for LLM interactions.
package privacy

import (
	"path/filepath"
	"strings"

	"github.com/belluikit/clair/kb"
	"github.com/bmatcuk/doublestar/v4"
)

// Mode controls what data can be sent to LLM providers.
type Mode int

const (
	// ModeLocal restricts to local-only LLMs. Nothing leaves the machine.
	ModeLocal Mode = iota

	// ModeFiltered allows remote LLMs but applies deny patterns to filter content.
	ModeFiltered

	// ModeOpen sends any data to the LLM (user's choice).
	ModeOpen
)

// Policy defines privacy rules for data flowing to LLM providers.
type Policy struct {
	// Mode is the privacy mode.
	Mode Mode

	// DenyPatterns are file path glob patterns that are never sent to remote LLMs.
	DenyPatterns []string

	// AllowPatterns override deny patterns for specific files.
	AllowPatterns []string
}

// DefaultPolicy returns a sensible default privacy policy.
func DefaultPolicy() Policy {
	return Policy{
		Mode: ModeFiltered,
		DenyPatterns: []string{
			"**/.env",
			"**/.env.*",
			"**/credentials*",
			"**/secrets*",
			"**/*_secret*",
			"**/private_key*",
			"**/*.pem",
			"**/*.key",
		},
	}
}

// Filter removes search results that should not be sent to the LLM
// based on the policy's deny/allow patterns.
func (p Policy) Filter(results []kb.SearchResult) []kb.SearchResult {
	if p.Mode == ModeOpen {
		return results
	}

	filtered := make([]kb.SearchResult, 0, len(results))
	for _, r := range results {
		if p.CanSendToRemote(r.FilePath) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// CanSendToRemote returns true if content from the given file path
// is allowed to be sent to a remote LLM.
func (p Policy) CanSendToRemote(filePath string) bool {
	if p.Mode == ModeOpen {
		return true
	}
	if p.Mode == ModeLocal {
		return false
	}

	if filePath == "" {
		return true // no file path means it's not file-sourced content
	}

	// Check allow patterns first (they override deny).
	for _, pattern := range p.AllowPatterns {
		if matched, _ := matchPath(pattern, filePath); matched {
			return true
		}
	}

	// Check deny patterns.
	for _, pattern := range p.DenyPatterns {
		if matched, _ := matchPath(pattern, filePath); matched {
			return false
		}
	}

	return true
}

// matchPath matches a glob pattern against a file path, trying both the
// full path and just the base name for simple patterns.
func matchPath(pattern, path string) (bool, error) {
	if matched, err := doublestar.Match(pattern, path); matched || err != nil {
		return matched, err
	}
	// Also try matching against just the filename for simple patterns.
	if !strings.Contains(pattern, "/") && !strings.Contains(pattern, string(filepath.Separator)) {
		base := filepath.Base(path)
		return doublestar.Match(pattern, base)
	}
	return false, nil
}
