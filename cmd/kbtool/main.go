// kbtool builds, inspects, and queries Clair knowledge base indexes.
//
// Usage:
//
//	kbtool build   --source <dir> --module <path> --output <path>
//	kbtool inspect <index-path>
//	kbtool search  <index-path> <query>
//	kbtool validate <index-path>
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/belluikit/clair/kb"
	"github.com/belluikit/clair/kb/builder"
	"github.com/belluikit/clair/parser/goparser"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "build":
		cmdBuild(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "search":
		cmdSearch(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `kbtool — Clair knowledge base tool

Commands:
  build     Build a framework index from source code
  inspect   Show statistics about an index
  search    Search an index
  validate  Check index for missing documentation

Run 'kbtool <command> -h' for details.`)
}

func cmdBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	source := fs.String("source", "", "Source directory to scan")
	module := fs.String("module", "", "Framework module path (for reference detection)")
	output := fs.String("output", "", "Output index path")
	fs.Parse(args)

	if *source == "" || *output == "" {
		fs.Usage()
		os.Exit(1)
	}

	b := builder.New()
	p := goparser.New(*module)

	fmt.Printf("Scanning %s...\n", *source)
	if err := b.AddFromParser(p, *source); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d symbols\n", b.DocCount())
	fmt.Printf("Building index at %s...\n", *output)

	if err := b.Build(*output); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done.")
}

func cmdInspect(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: kbtool inspect <index-path>")
		os.Exit(1)
	}

	idx, err := kb.OpenFramework(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer idx.Close()

	// Search all docs.
	results, err := idx.Search("*", 10000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Index: %s\n", args[0])
	fmt.Printf("Documents: %d\n", len(results))

	// Count by kind.
	kinds := make(map[string]int)
	packages := make(map[string]int)
	for _, r := range results {
		kinds[r.Kind.String()]++
		packages[r.Package]++
	}

	fmt.Println("\nBy kind:")
	for k, c := range kinds {
		fmt.Printf("  %-12s %d\n", k, c)
	}

	fmt.Println("\nBy package:")
	for p, c := range packages {
		fmt.Printf("  %-20s %d\n", p, c)
	}
}

func cmdSearch(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kbtool search <index-path> <query>")
		os.Exit(1)
	}

	indexPath := args[0]
	query := strings.Join(args[1:], " ")

	idx, err := kb.OpenFramework(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer idx.Close()

	results, err := idx.Search(query, 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	for i, r := range results {
		fmt.Printf("%d. %s (%.2f)\n", i+1, r.ID, r.Score)
		fmt.Printf("   Kind: %s  Package: %s\n", r.Kind, r.Package)
		if r.Signature != "" {
			fmt.Printf("   %s\n", r.Signature)
		}
		if r.Doc != "" {
			doc := r.Doc
			if len(doc) > 120 {
				doc = doc[:117] + "..."
			}
			fmt.Printf("   %s\n", doc)
		}
		fmt.Println()
	}
}

func cmdValidate(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: kbtool validate <index-path>")
		os.Exit(1)
	}

	idx, err := kb.OpenFramework(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer idx.Close()

	results, err := idx.Search("*", 10000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	total := len(results)
	missingDoc := 0
	missingExample := 0
	var issues []string

	for _, r := range results {
		if r.Doc == "" {
			missingDoc++
			issues = append(issues, fmt.Sprintf("  MISSING DOC: %s (%s)", r.ID, r.Kind))
		}
		if r.Example == "" {
			missingExample++
		}
	}

	fmt.Printf("Index: %s\n", args[0])
	fmt.Printf("Total symbols: %d\n", total)
	fmt.Printf("With documentation: %d (%.0f%%)\n", total-missingDoc, float64(total-missingDoc)/float64(total)*100)
	fmt.Printf("With examples: %d (%.0f%%)\n", total-missingExample, float64(total-missingExample)/float64(total)*100)

	if len(issues) > 0 {
		fmt.Printf("\nSymbols missing documentation (%d):\n", missingDoc)
		for _, issue := range issues {
			fmt.Println(issue)
		}
	}

	if missingDoc > 0 {
		os.Exit(1)
	}
}
