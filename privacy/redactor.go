package privacy

import "regexp"

// Redactor detects and masks sensitive patterns in text.
type Redactor struct {
	patterns []*regexp.Regexp
}

// DefaultRedactor creates a Redactor with common secret patterns.
func DefaultRedactor() *Redactor {
	return &Redactor{
		patterns: []*regexp.Regexp{
			// API keys and tokens (generic patterns).
			regexp.MustCompile(`(?i)(api[_-]?key|api[_-]?secret|api[_-]?token)\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}["']?`),
			regexp.MustCompile(`(?i)(access[_-]?token|auth[_-]?token|bearer)\s*[:=]\s*["']?[A-Za-z0-9_\-\.]{16,}["']?`),
			regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["']?[^\s"']{4,}["']?`),
			regexp.MustCompile(`(?i)(secret[_-]?key|private[_-]?key)\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}["']?`),

			// Specific provider patterns.
			regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),                                   // OpenAI
			regexp.MustCompile(`sk-ant-[A-Za-z0-9\-]{20,}`),                             // Anthropic
			regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),                                   // GitHub PAT
			regexp.MustCompile(`glpat-[A-Za-z0-9\-]{20,}`),                              // GitLab PAT
			regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                                      // AWS access key
			regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*[^\s]+`), // AWS secret

			// Environment variable assignments with sensitive names.
			regexp.MustCompile(`(?i)^(export\s+)?(DB_PASSWORD|DATABASE_URL|REDIS_URL|MONGO_URI)\s*=\s*.*$`),
		},
	}
}

// Redact replaces detected sensitive patterns with [REDACTED].
func (r *Redactor) Redact(text string) string {
	for _, p := range r.patterns {
		text = p.ReplaceAllString(text, "[REDACTED]")
	}
	return text
}

// ContainsSensitive returns true if the text contains any sensitive patterns.
func (r *Redactor) ContainsSensitive(text string) bool {
	for _, p := range r.patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}
