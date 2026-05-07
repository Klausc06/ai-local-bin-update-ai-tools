package redactor

import (
	"strings"
	"testing"
)

func TestRedactsURLQuerySecrets(t *testing.T) {
	red := New()
	input := "https://ai.variflight.com/mcp/?api_key=sk-Ab49TufyGobbr4cVPqEIqJcExO53fRsGL7YcwFqLzok&plain=ok"
	out := red.Redact(input)
	if strings.Contains(out, "Ab49Tufy") || strings.Contains(out, "Lzok") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "plain=ok") {
		t.Fatalf("non-secret query was unexpectedly removed: %s", out)
	}
}

func TestRedactsFieldsAndPhone(t *testing.T) {
	red := New()
	input := `{"api_key":"abc123456789","Authorization":"Bearer deadbeef123456","phone":"13800138000"}`
	out := red.Redact(input)
	for _, leaked := range []string{"abc123456789", "deadbeef123456", "13800138000"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("sensitive value leaked: %s", out)
		}
	}
}

func TestRedactsBearerToken(t *testing.T) {
	red := New()
	out := red.Redact("Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.dGhpcyBpcyBhIHRlc3Q")
	if strings.Contains(out, "eyJhbGci") {
		t.Fatalf("Bearer token leaked: %s", out)
	}
	if !strings.Contains(out, "*****") {
		t.Fatalf("expected redacted output, got: %s", out)
	}
}

func TestRedactsEnvFileFormat(t *testing.T) {
	red := New()
	out := red.Redact("GITHUB_TOKEN=ghp_abc123def456\nOPENAI_API_KEY=sk-xyz789")
	for _, leaked := range []string{"ghp_abc123def456", "sk-xyz789"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("env value leaked: %s", out)
		}
	}
}
