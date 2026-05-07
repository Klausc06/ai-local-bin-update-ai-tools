package report

import "testing"

func TestDeduplicateRisks(t *testing.T) {
	risks := []Risk{
		{Provider: "codex", Name: "spotify", Level: "manual", Reason: "manual"},
		{Provider: "codex", Name: "spotify", Level: "manual", Reason: "manual"},
		{Provider: "mcp", Name: "spotify", Level: "manual", Reason: "manual"},
	}
	got := DeduplicateRisks(risks)
	if len(got) != 2 {
		t.Fatalf("expected 2 risks, got %d", len(got))
	}
}
