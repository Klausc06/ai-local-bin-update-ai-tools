package runner

import "testing"

func TestFirstSignificantLineSkipsWarnings(t *testing.T) {
	got := firstSignificantLine("WARNING: noisy\ncodex-cli 0.128.0", "fallback")
	if got != "codex-cli 0.128.0" {
		t.Fatalf("got %q", got)
	}
}

func TestLooksLikeHealthWarning(t *testing.T) {
	if !looksLikeHealthWarning("plugin:playwright - ✗ Failed to connect") {
		t.Fatal("expected health warning")
	}
}

func TestShouldTryFallback(t *testing.T) {
	if !shouldTryFallback("error: unrecognized command update") {
		t.Fatal("expected fallback")
	}
	if shouldTryFallback("network timeout") {
		t.Fatal("did not expect fallback")
	}
}
