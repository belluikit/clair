package privacy

import (
	"testing"

	"github.com/belluikit/clair/kb"
	"github.com/belluikit/clair/parser"
)

func TestPolicy_CanSendToRemote(t *testing.T) {
	p := DefaultPolicy()

	tests := []struct {
		path string
		want bool
	}{
		{"src/main.go", true},
		{".env", false},
		{".env.production", false},
		{"config/credentials.json", false},
		{"internal/secrets.yaml", false},
		{"certs/server.pem", false},
		{"certs/server.key", false},
		{"src/widget.go", true},
		{"", true}, // no path
	}

	for _, tt := range tests {
		got := p.CanSendToRemote(tt.path)
		if got != tt.want {
			t.Errorf("CanSendToRemote(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestPolicy_ModeLocal(t *testing.T) {
	p := Policy{Mode: ModeLocal}
	if p.CanSendToRemote("anything.go") {
		t.Error("ModeLocal should deny all")
	}
}

func TestPolicy_ModeOpen(t *testing.T) {
	p := Policy{Mode: ModeOpen}
	if !p.CanSendToRemote(".env") {
		t.Error("ModeOpen should allow all")
	}
}

func TestPolicy_Filter(t *testing.T) {
	p := DefaultPolicy()

	results := []kb.SearchResult{
		{Document: kb.Document{ID: "1", FilePath: "src/main.go"}},
		{Document: kb.Document{ID: "2", FilePath: ".env"}},
		{Document: kb.Document{ID: "3", FilePath: "internal/service.go"}},
	}

	filtered := p.Filter(results)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 results, got %d", len(filtered))
	}
	if filtered[0].ID != "1" || filtered[1].ID != "3" {
		t.Error("wrong results filtered")
	}
}

func TestPolicy_AllowOverridesDeny(t *testing.T) {
	p := Policy{
		Mode:          ModeFiltered,
		DenyPatterns:  []string{"**/*.yaml"},
		AllowPatterns: []string{"**/config.yaml"},
	}

	if p.CanSendToRemote("secrets.yaml") {
		t.Error("secrets.yaml should be denied")
	}
	if !p.CanSendToRemote("config.yaml") {
		t.Error("config.yaml should be allowed (override)")
	}
}

func TestRedactor_Redact(t *testing.T) {
	r := DefaultRedactor()

	tests := []struct {
		name  string
		input string
		clean bool // true if no sensitive content expected
	}{
		{"normal code", "func main() { fmt.Println(\"hello\") }", true},
		{"openai key", "key := \"sk-abc123def456ghi789jkl012mno345pqr\"", false},
		{"anthropic key", "sk-ant-abc123-def456ghi789jkl012mno345", false},
		{"github pat", "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12", false},
		{"aws key", "AKIAIOSFODNN7EXAMPLE", false},
		{"api key assignment", "API_KEY=my_super_secret_key_12345", false},
		{"password", "password=hunter2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasSensitive := r.ContainsSensitive(tt.input)
			if tt.clean && hasSensitive {
				t.Errorf("false positive: %q flagged as sensitive", tt.input)
			}
			if !tt.clean && !hasSensitive {
				t.Errorf("missed sensitive content in: %q", tt.input)
			}

			if !tt.clean {
				redacted := r.Redact(tt.input)
				if r.ContainsSensitive(redacted) {
					t.Errorf("redaction incomplete: %q", redacted)
				}
			}
		})
	}
}

// Ensure types compile correctly.
var _ = kb.Document{Kind: parser.KindFunc}
