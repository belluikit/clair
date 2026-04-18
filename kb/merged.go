package kb

import (
	"sort"
	"strings"
	"sync"
)

// MergedKB provides unified search across framework and project knowledge bases.
// Results are ranked with tier-aware boosting and deduplication.
type MergedKB struct {
	Framework *FrameworkKB
	Project   *ProjectKB

	// ProjectBoost is a score multiplier for project results (default 1.2).
	// Project symbols are typically more relevant since they represent the developer's own code.
	ProjectBoost float64

	// ReferenceBoost is a bonus added to framework results whose symbols
	// appear in project references, and vice versa (default 0.5).
	ReferenceBoost float64
}

// NewMergedKB creates a MergedKB with default boost values.
func NewMergedKB(framework *FrameworkKB, project *ProjectKB) *MergedKB {
	return &MergedKB{
		Framework:      framework,
		Project:        project,
		ProjectBoost:   1.2,
		ReferenceBoost: 0.5,
	}
}

// LookupSymbol tries both KBs, preferring project over framework.
func (m *MergedKB) LookupSymbol(name string) (*Document, error) {
	if m.Project != nil {
		doc, err := m.Project.LookupSymbol(name)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}
	if m.Framework != nil {
		return m.Framework.LookupSymbol(name)
	}
	return nil, nil
}

// Search queries both tiers concurrently and merges results.
func (m *MergedKB) Search(query string, limit int) ([]SearchResult, error) {
	var (
		fwResults, projResults []SearchResult
		fwErr, projErr         error
		wg                     sync.WaitGroup
	)

	// Search both KBs concurrently.
	if m.Framework != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fwResults, fwErr = m.Framework.Search(query, limit)
		}()
	}

	if m.Project != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			projResults, projErr = m.Project.Search(query, limit)
		}()
	}

	wg.Wait()

	if fwErr != nil {
		return nil, fwErr
	}
	if projErr != nil {
		return nil, projErr
	}

	// Apply tier-specific boosts.
	for i := range projResults {
		projResults[i].Score *= m.ProjectBoost
	}

	// Cross-reference boost: boost framework results for types referenced by project,
	// and project results that reference framework types matching the query.
	m.applyReferenceBoosts(fwResults, projResults, query)

	// Merge and deduplicate.
	merged := append(fwResults, projResults...)
	merged = dedup(merged)

	// Sort by score descending.
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged, nil
}

// applyReferenceBoosts boosts scores based on cross-tier references.
func (m *MergedKB) applyReferenceBoosts(fwResults, projResults []SearchResult, query string) {
	if m.ReferenceBoost == 0 {
		return
	}

	// Collect framework types referenced by project results.
	projRefs := make(map[string]bool)
	for _, r := range projResults {
		for _, ref := range r.References {
			projRefs[ref] = true
		}
	}

	// Boost framework results that are referenced by project code.
	for i, r := range fwResults {
		key := r.Package + "." + r.Symbol
		if projRefs[key] {
			fwResults[i].Score += m.ReferenceBoost
		}
	}

	// Boost project results that reference framework types mentioned in the query.
	queryLower := strings.ToLower(query)
	for i, r := range projResults {
		for _, ref := range r.References {
			if strings.Contains(queryLower, strings.ToLower(ref)) ||
				strings.Contains(queryLower, strings.ToLower(refBaseName(ref))) {
				projResults[i].Score += m.ReferenceBoost
				break
			}
		}
	}
}

// refBaseName returns the type name part of a reference ("widget.List" → "List").
func refBaseName(ref string) string {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// dedup removes duplicate search results by ID, keeping the highest-scored version.
func dedup(results []SearchResult) []SearchResult {
	seen := make(map[string]int, len(results))
	out := make([]SearchResult, 0, len(results))

	for _, r := range results {
		if idx, exists := seen[r.ID]; exists {
			if r.Score > out[idx].Score {
				out[idx] = r
			}
		} else {
			seen[r.ID] = len(out)
			out = append(out, r)
		}
	}

	return out
}

// Close closes both underlying indexes.
func (m *MergedKB) Close() error {
	var firstErr error
	if m.Framework != nil {
		if err := m.Framework.Close(); err != nil {
			firstErr = err
		}
	}
	if m.Project != nil {
		if err := m.Project.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
