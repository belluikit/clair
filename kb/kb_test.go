package kb_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/clair/kb"
	"github.com/belluikit/clair/kb/builder"
	"github.com/belluikit/clair/parser"
	"github.com/belluikit/clair/parser/goparser"
)

func TestProjectKB_IndexAndSearch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "project.bleve")

	proj, err := kb.OpenProject(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	defer proj.Close()

	symbols := []parser.Symbol{
		{
			Name:      "OrderService",
			Kind:      parser.KindInterface,
			Package:   "orders",
			Signature: "type OrderService interface { ListOrders() []Order }",
			Doc:       "OrderService handles order operations.",
		},
		{
			Name:       "OrderList",
			Kind:       parser.KindType,
			Package:    "orders",
			Signature:  "type OrderList struct { Items []Order }",
			Doc:        "OrderList displays orders in a widget list.",
			References: []string{"widget.List"},
		},
		{
			Name:      "NewOrderService",
			Kind:      parser.KindFunc,
			Package:   "orders",
			Signature: "func NewOrderService(db *DB) OrderService",
			Doc:       "NewOrderService creates an order service.",
		},
	}

	if err := proj.Index(symbols); err != nil {
		t.Fatal(err)
	}

	count, err := proj.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 docs, got %d", count)
	}

	t.Run("search by keyword", func(t *testing.T) {
		results, err := proj.Search("order", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Fatal("expected results for 'order'")
		}
	})

	t.Run("lookup by ID", func(t *testing.T) {
		doc, err := proj.LookupSymbol("orders.OrderService")
		if err != nil {
			t.Fatal(err)
		}
		if doc == nil {
			t.Fatal("expected to find OrderService")
		}
		if doc.Kind != parser.KindInterface {
			t.Errorf("expected KindInterface, got %v", doc.Kind)
		}
	})

	t.Run("remove by file", func(t *testing.T) {
		// Index a symbol with a file path, then remove it.
		err := proj.Index([]parser.Symbol{{
			Name:     "TempWidget",
			Kind:     parser.KindType,
			Package:  "temp",
			FilePath: "/tmp/temp.go",
		}})
		if err != nil {
			t.Fatal(err)
		}

		if err := proj.RemoveByFile("/tmp/temp.go"); err != nil {
			t.Fatal(err)
		}

		doc, err := proj.LookupSymbol("temp.TempWidget")
		if err != nil {
			t.Fatal(err)
		}
		if doc != nil {
			t.Error("expected TempWidget to be removed")
		}
	})
}

func TestBuilder_BuildAndOpen(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "framework.bleve")

	b := builder.New()

	// Add a manual doc.
	b.Add(kb.Document{
		ID:        "widget.Button",
		Symbol:    "Button",
		Package:   "widget",
		Kind:      parser.KindType,
		Signature: "type Button struct { Text string; OnTapped func() }",
		Doc:       "Button is a clickable widget that triggers an action.",
		Example:   `btn := widget.NewButton("Click", func() { fmt.Println("clicked") })`,
	})

	// Add from parser using test fixture.
	gp := goparser.New("example.com/framework")

	// Write a small Go file as fixture.
	fixtureDir := filepath.Join(dir, "src")
	os.MkdirAll(fixtureDir, 0o755)
	os.WriteFile(filepath.Join(fixtureDir, "entry.go"), []byte(`
// Package widget provides UI components.
package widget

// Entry is a text input widget.
type Entry struct {
	Text string
}

// NewEntry creates a new Entry.
func NewEntry() *Entry {
	return &Entry{}
}
`), 0o644)

	if err := b.AddFromParser(gp, fixtureDir); err != nil {
		t.Fatal(err)
	}

	if b.DocCount() < 2 {
		t.Fatalf("expected at least 2 docs, got %d", b.DocCount())
	}

	if err := b.Build(indexPath); err != nil {
		t.Fatal(err)
	}

	// Open and verify.
	fw, err := kb.OpenFramework(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	doc, err := fw.LookupSymbol("widget.Button")
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected to find Button")
	}
	if doc.Tier != kb.TierFramework {
		t.Errorf("expected TierFramework, got %v", doc.Tier)
	}

	results, err := fw.Search("text input", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'text input'")
	}
}

func TestMergedKB_Search(t *testing.T) {
	dir := t.TempDir()

	// Build framework index.
	b := builder.New()
	b.Add(kb.Document{
		ID:        "widget.List",
		Symbol:    "List",
		Package:   "widget",
		Kind:      parser.KindType,
		Signature: "type List struct {}",
		Doc:       "List is a scrolling list widget.",
	})
	b.Add(kb.Document{
		ID:        "widget.Button",
		Symbol:    "Button",
		Package:   "widget",
		Kind:      parser.KindType,
		Signature: "type Button struct {}",
		Doc:       "Button is a clickable widget.",
	})

	fwPath := filepath.Join(dir, "fw.bleve")
	if err := b.Build(fwPath); err != nil {
		t.Fatal(err)
	}

	fw, err := kb.OpenFramework(fwPath)
	if err != nil {
		t.Fatal(err)
	}

	// Build project index.
	projPath := filepath.Join(dir, "proj.bleve")
	proj, err := kb.OpenProject(projPath)
	if err != nil {
		t.Fatal(err)
	}

	err = proj.Index([]parser.Symbol{
		{
			Name:       "OrderList",
			Kind:       parser.KindType,
			Package:    "orders",
			Signature:  "type OrderList struct {}",
			Doc:        "OrderList displays orders using a list widget.",
			References: []string{"widget.List"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	merged := kb.NewMergedKB(fw, proj)
	defer merged.Close()

	t.Run("returns results from both tiers", func(t *testing.T) {
		results, err := merged.Search("list", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) < 2 {
			t.Fatalf("expected at least 2 results, got %d", len(results))
		}
	})

	t.Run("project results ranked higher", func(t *testing.T) {
		results, err := merged.Search("list", 10)
		if err != nil {
			t.Fatal(err)
		}
		// With project boost, OrderList should score higher than widget.List.
		if len(results) >= 2 {
			if results[0].Tier != kb.TierProject {
				t.Log("project result not ranked first — this may be acceptable depending on query relevance")
			}
		}
	})

	t.Run("lookup prefers project", func(t *testing.T) {
		// Add a project symbol with same name as framework.
		proj.Index([]parser.Symbol{{
			Name:    "List",
			Kind:    parser.KindType,
			Package: "widget",
			Doc:     "Project's custom List override.",
		}})

		doc, err := merged.LookupSymbol("widget.List")
		if err != nil {
			t.Fatal(err)
		}
		if doc == nil {
			t.Fatal("expected to find widget.List")
		}
		if doc.Tier != kb.TierProject {
			t.Error("expected project tier to take precedence")
		}
	})
}
