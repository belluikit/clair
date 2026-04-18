package goparser

import (
	"os"
	"testing"

	"github.com/trendvidia/clair/parser"
)

func TestExtensions(t *testing.T) {
	p := New("example.com/framework")
	exts := p.Extensions()
	if len(exts) != 1 || exts[0] != ".go" {
		t.Fatalf("expected [\".go\"], got %v", exts)
	}
}

func TestParseFile(t *testing.T) {
	content, err := os.ReadFile("testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}

	p := New("example.com/framework")
	symbols, err := p.ParseFile("testdata/sample.go", content)
	if err != nil {
		t.Fatal(err)
	}

	byName := make(map[string]parser.Symbol)
	for _, s := range symbols {
		byName[s.ID()] = s
	}

	t.Run("exported type", func(t *testing.T) {
		sym, ok := byName["sample.Widget"]
		if !ok {
			t.Fatal("expected Widget type")
		}
		if sym.Kind != parser.KindType {
			t.Errorf("expected KindType, got %v", sym.Kind)
		}
		if sym.Doc == "" {
			t.Error("expected doc comment")
		}
		if sym.Package != "sample" {
			t.Errorf("expected package sample, got %s", sym.Package)
		}
	})

	t.Run("interface", func(t *testing.T) {
		sym, ok := byName["sample.Renderer"]
		if !ok {
			t.Fatal("expected Renderer interface")
		}
		if sym.Kind != parser.KindInterface {
			t.Errorf("expected KindInterface, got %v", sym.Kind)
		}
	})

	t.Run("const", func(t *testing.T) {
		sym, ok := byName["sample.MaxItems"]
		if !ok {
			t.Fatal("expected MaxItems const")
		}
		if sym.Kind != parser.KindConst {
			t.Errorf("expected KindConst, got %v", sym.Kind)
		}
	})

	t.Run("var", func(t *testing.T) {
		sym, ok := byName["sample.DefaultName"]
		if !ok {
			t.Fatal("expected DefaultName var")
		}
		if sym.Kind != parser.KindVar {
			t.Errorf("expected KindVar, got %v", sym.Kind)
		}
	})

	t.Run("function", func(t *testing.T) {
		sym, ok := byName["sample.NewWidget"]
		if !ok {
			t.Fatal("expected NewWidget func")
		}
		if sym.Kind != parser.KindFunc {
			t.Errorf("expected KindFunc, got %v", sym.Kind)
		}
		if len(sym.References) == 0 {
			t.Error("expected framework references for NewWidget")
		}
	})

	t.Run("method", func(t *testing.T) {
		sym, ok := byName["Widget.Render"]
		if !ok {
			t.Fatal("expected Widget.Render method")
		}
		if sym.Kind != parser.KindMethod {
			t.Errorf("expected KindMethod, got %v", sym.Kind)
		}
		if sym.Receiver != "Widget" {
			t.Errorf("expected receiver Widget, got %s", sym.Receiver)
		}
	})

	t.Run("unexported symbols excluded", func(t *testing.T) {
		for _, name := range []string{"sample.unexportedConst", "sample.unexportedFunc", "sample.internal"} {
			if _, ok := byName[name]; ok {
				t.Errorf("unexported symbol %q should not be extracted", name)
			}
		}
	})

	t.Run("framework references on struct", func(t *testing.T) {
		sym := byName["sample.Widget"]
		found := false
		for _, ref := range sym.References {
			if ref == "widget.List" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected reference to widget.List, got %v", sym.References)
		}
	})
}

func TestParseFile_InvalidSource(t *testing.T) {
	p := New("example.com/framework")
	symbols, err := p.ParseFile("bad.go", []byte("this is not valid go"))
	if err != nil {
		t.Fatal("expected nil error for invalid source")
	}
	if symbols != nil {
		t.Fatal("expected nil symbols for invalid source")
	}
}

func TestParseFile_NoFrameworkModule(t *testing.T) {
	content, err := os.ReadFile("testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}

	p := New("") // no framework module
	symbols, err := p.ParseFile("testdata/sample.go", content)
	if err != nil {
		t.Fatal(err)
	}

	for _, sym := range symbols {
		if len(sym.References) > 0 {
			t.Errorf("expected no references with empty framework module, got %v for %s", sym.References, sym.Name)
		}
	}
}
