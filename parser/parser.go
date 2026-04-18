package parser

// Parser extracts symbols from source code files.
type Parser interface {
	// Extensions returns the file extensions this parser handles (e.g. [".go"]).
	Extensions() []string

	// ParseFile analyzes a source file and returns the symbols found.
	// Implementations should return nil (not an error) for unparseable files
	// to avoid blocking indexing of other files.
	ParseFile(path string, content []byte) ([]Symbol, error)
}
