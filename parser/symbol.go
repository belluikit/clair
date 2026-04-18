// Package parser defines interfaces and types for source code analysis.
package parser

// SymbolKind represents the kind of a code symbol.
type SymbolKind int

const (
	KindType      SymbolKind = iota // struct, alias
	KindInterface                   // interface
	KindFunc                        // top-level function
	KindMethod                      // method with receiver
	KindConst                       // constant
	KindVar                         // variable
)

func (k SymbolKind) String() string {
	switch k {
	case KindType:
		return "type"
	case KindInterface:
		return "interface"
	case KindFunc:
		return "func"
	case KindMethod:
		return "method"
	case KindConst:
		return "const"
	case KindVar:
		return "var"
	default:
		return "unknown"
	}
}

// Symbol represents a parsed code symbol extracted from source.
type Symbol struct {
	// Name is the symbol's identifier (e.g. "Widget", "NewWidget", "Render").
	Name string

	// Kind is the symbol classification.
	Kind SymbolKind

	// Package is the package name containing this symbol.
	Package string

	// Signature is the full type or function signature.
	// For types: "type Widget struct { ... }"
	// For funcs: "func NewWidget(opts ...Option) *Widget"
	Signature string

	// Doc is the godoc comment associated with this symbol.
	Doc string

	// Receiver is the receiver type for methods (empty for non-methods).
	Receiver string

	// References lists framework types used by this symbol.
	// Format: "package.Type" (e.g. "widget.List", "binding.String").
	References []string

	// FilePath is the source file where the symbol is defined.
	FilePath string

	// Line is the line number of the symbol declaration.
	Line int
}

// ID returns a unique identifier for the symbol.
// Methods are prefixed with their receiver: "Widget.Render".
// Other symbols use package-qualified names: "widget.NewWidget".
func (s Symbol) ID() string {
	if s.Receiver != "" {
		return s.Receiver + "." + s.Name
	}
	if s.Package != "" {
		return s.Package + "." + s.Name
	}
	return s.Name
}
