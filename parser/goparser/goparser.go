// Package goparser implements a Go source code parser that extracts symbols using go/ast.
package goparser

import (
	"bytes"
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"strings"

	"github.com/trendvidia/clair/parser"
)

// GoParser extracts symbols from Go source files.
type GoParser struct {
	// frameworkModule is the module path prefix used to detect framework type references.
	// e.g. "fyne.io/fyne/v2" — any import starting with this prefix is treated as a framework import.
	frameworkModule string
}

// New creates a GoParser. frameworkModule is the module path prefix for detecting
// framework type references (e.g. "fyne.io/fyne/v2"). Pass "" to disable reference detection.
func New(frameworkModule string) *GoParser {
	return &GoParser{frameworkModule: frameworkModule}
}

func (p *GoParser) Extensions() []string {
	return []string{".go"}
}

func (p *GoParser) ParseFile(path string, content []byte) ([]parser.Symbol, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, path, content, goparser.ParseComments)
	if err != nil {
		// Return nil for unparseable files — don't block indexing.
		return nil, nil
	}

	pkgName := file.Name.Name

	// Build a map of framework import aliases: alias → full import path.
	frameworkImports := p.resolveFrameworkImports(file)

	var symbols []parser.Symbol

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			syms := p.extractGenDecl(d, pkgName, path, fset, frameworkImports)
			symbols = append(symbols, syms...)
		case *ast.FuncDecl:
			if sym, ok := p.extractFuncDecl(d, pkgName, path, fset, frameworkImports); ok {
				symbols = append(symbols, sym)
			}
		}
	}

	return symbols, nil
}

// resolveFrameworkImports returns a map of local alias → full import path for
// imports that match the configured framework module prefix.
func (p *GoParser) resolveFrameworkImports(file *ast.File) map[string]string {
	if p.frameworkModule == "" {
		return nil
	}

	imports := make(map[string]string)
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if !strings.HasPrefix(importPath, p.frameworkModule) {
			continue
		}

		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			// Default alias is the last path segment.
			parts := strings.Split(importPath, "/")
			alias = parts[len(parts)-1]
		}
		imports[alias] = importPath
	}
	return imports
}

// extractGenDecl extracts symbols from a general declaration (type, const, var).
func (p *GoParser) extractGenDecl(decl *ast.GenDecl, pkg, filePath string, fset *token.FileSet, fwImports map[string]string) []parser.Symbol {
	var symbols []parser.Symbol

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if !ast.IsExported(s.Name.Name) {
				continue
			}
			sym := parser.Symbol{
				Name:     s.Name.Name,
				Package:  pkg,
				FilePath: filePath,
				Line:     fset.Position(s.Pos()).Line,
				Doc:      extractDoc(decl.Doc, s.Doc),
			}

			switch typ := s.Type.(type) {
			case *ast.InterfaceType:
				sym.Kind = parser.KindInterface
				sym.Signature = formatInterfaceSignature(s.Name.Name, typ, fset)
				sym.References = collectFieldListRefs(typ.Methods, fwImports)
			case *ast.StructType:
				sym.Kind = parser.KindType
				sym.Signature = formatStructSignature(s.Name.Name, typ, fset)
				sym.References = collectFieldListRefs(typ.Fields, fwImports)
			default:
				sym.Kind = parser.KindType
				sym.Signature = "type " + s.Name.Name + " " + nodeString(fset, s.Type)
				sym.References = collectExprRefs(s.Type, fwImports)
			}

			symbols = append(symbols, sym)

		case *ast.ValueSpec:
			for _, name := range s.Names {
				if !ast.IsExported(name.Name) {
					continue
				}
				sym := parser.Symbol{
					Name:     name.Name,
					Package:  pkg,
					FilePath: filePath,
					Line:     fset.Position(name.Pos()).Line,
					Doc:      extractDoc(decl.Doc, s.Doc),
				}

				switch decl.Tok {
				case token.CONST:
					sym.Kind = parser.KindConst
					if s.Type != nil {
						sym.Signature = "const " + name.Name + " " + nodeString(fset, s.Type)
					} else {
						sym.Signature = "const " + name.Name
					}
				case token.VAR:
					sym.Kind = parser.KindVar
					if s.Type != nil {
						sym.Signature = "var " + name.Name + " " + nodeString(fset, s.Type)
						sym.References = collectExprRefs(s.Type, fwImports)
					} else {
						sym.Signature = "var " + name.Name
					}
				}

				symbols = append(symbols, sym)
			}
		}
	}

	return symbols
}

// extractFuncDecl extracts a symbol from a function or method declaration.
func (p *GoParser) extractFuncDecl(decl *ast.FuncDecl, pkg, filePath string, fset *token.FileSet, fwImports map[string]string) (parser.Symbol, bool) {
	if !ast.IsExported(decl.Name.Name) {
		return parser.Symbol{}, false
	}

	sym := parser.Symbol{
		Name:     decl.Name.Name,
		Package:  pkg,
		FilePath: filePath,
		Line:     fset.Position(decl.Pos()).Line,
		Doc:      extractDoc(decl.Doc, nil),
	}

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		sym.Kind = parser.KindMethod
		sym.Receiver = receiverTypeName(decl.Recv.List[0].Type)
		sym.References = collectFieldListRefs(decl.Recv, fwImports)
	} else {
		sym.Kind = parser.KindFunc
	}

	// Build signature.
	sym.Signature = formatFuncSignature(decl, fset)

	// Collect references from params and results.
	if decl.Type.Params != nil {
		sym.References = append(sym.References, collectFieldListRefs(decl.Type.Params, fwImports)...)
	}
	if decl.Type.Results != nil {
		sym.References = append(sym.References, collectFieldListRefs(decl.Type.Results, fwImports)...)
	}

	sym.References = dedup(sym.References)

	return sym, true
}

// extractDoc returns the documentation text from comment groups.
// It prefers the spec-level doc over the decl-level doc.
func extractDoc(declDoc, specDoc *ast.CommentGroup) string {
	if specDoc != nil {
		return strings.TrimSpace(specDoc.Text())
	}
	if declDoc != nil {
		return strings.TrimSpace(declDoc.Text())
	}
	return ""
}

// receiverTypeName extracts the type name from a method receiver expression,
// stripping pointer indirection.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	default:
		return ""
	}
}

// collectFieldListRefs collects framework type references from a field list.
func collectFieldListRefs(fl *ast.FieldList, fwImports map[string]string) []string {
	if fl == nil || len(fwImports) == 0 {
		return nil
	}
	var refs []string
	for _, field := range fl.List {
		refs = append(refs, collectExprRefs(field.Type, fwImports)...)
	}
	return refs
}

// collectExprRefs collects framework type references from an expression.
func collectExprRefs(expr ast.Expr, fwImports map[string]string) []string {
	if len(fwImports) == 0 {
		return nil
	}
	var refs []string
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, isFW := fwImports[ident.Name]; isFW {
			refs = append(refs, ident.Name+"."+sel.Sel.Name)
		}
		return true
	})
	return refs
}

// formatFuncSignature produces a function signature string.
func formatFuncSignature(decl *ast.FuncDecl, fset *token.FileSet) string {
	// Print just the declaration without the body.
	clone := *decl
	clone.Body = nil
	return nodeString(fset, &clone)
}

// formatStructSignature produces a compact struct type signature.
func formatStructSignature(name string, st *ast.StructType, fset *token.FileSet) string {
	return "type " + name + " " + nodeString(fset, st)
}

// formatInterfaceSignature produces a compact interface type signature.
func formatInterfaceSignature(name string, iface *ast.InterfaceType, fset *token.FileSet) string {
	return "type " + name + " " + nodeString(fset, iface)
}

// nodeString renders an AST node to its Go source representation.
func nodeString(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

// dedup removes duplicate strings from a slice, preserving order.
func dedup(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
