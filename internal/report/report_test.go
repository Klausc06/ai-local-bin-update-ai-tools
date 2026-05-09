package report

import (
	"bytes"
	"strings"
	"testing"

	"update-ai-tools/internal/redactor"
)

func TestDeduplicateRisks(t *testing.T) {
	risks := []Risk{
		{Provider: "codex", Name: "spotify", Level: "manual", Reason: "manual"},
		{Provider: "codex", Name: "spotify", Level: "manual", Reason: "manual"},
		{Provider: "mcp", Name: "xhs", Level: "manual", Reason: "different"},
	}
	got := DeduplicateRisks(risks)
	if len(got) != 2 {
		t.Fatalf("expected 2 risks (different names), got %d", len(got))
	}
}

func TestDeduplicateRisksEmpty(t *testing.T) {
	got := DeduplicateRisks(nil)
	if len(got) != 0 {
		t.Fatalf("expected 0 risks, got %d", len(got))
	}
}

func TestSummarizeCounts(t *testing.T) {
	results := []TaskResult{
		{Name: "a", Status: StatusSuccess},
		{Name: "b", Status: StatusSuccess},
		{Name: "c", Status: StatusFailed},
		{Name: "d", Status: StatusSkipped},
		{Name: "e", Status: StatusSkipped},
		{Name: "f", Status: StatusWarning},
		{Name: "g", Status: StatusInfo},
	}
	s := Summarize(results)
	if s.Success != 2 {
		t.Errorf("expected 2 success, got %d", s.Success)
	}
	if s.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", s.Failed)
	}
	if s.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", s.Skipped)
	}
	if s.Warning != 1 {
		t.Errorf("expected 1 warning, got %d", s.Warning)
	}
	if s.Info != 1 {
		t.Errorf("expected 1 info, got %d", s.Info)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil)
	if s.Success != 0 || s.Failed != 0 || s.Skipped != 0 || s.Warning != 0 || s.Info != 0 {
		t.Fatal("expected all zeros for empty results")
	}
}

func TestLoggerInfofVerbose(t *testing.T) {
	var fileBuf, consoleBuf bytes.Buffer
	red := redactor.New()
	log := NewLogger(&fileBuf, &consoleBuf, red, true)
	log.Infof("test message %d", 42)
	if !strings.Contains(fileBuf.String(), "test message 42") {
		t.Errorf("file buffer missing message: %q", fileBuf.String())
	}
	if !strings.Contains(consoleBuf.String(), "test message 42") {
		t.Errorf("verbose=true should write info to console: %q", consoleBuf.String())
	}
}

func TestLoggerInfofNotVerbose(t *testing.T) {
	var fileBuf, consoleBuf bytes.Buffer
	red := redactor.New()
	log := NewLogger(&fileBuf, &consoleBuf, red, false)
	log.Infof("test message %d", 42)
	if !strings.Contains(fileBuf.String(), "test message 42") {
		t.Errorf("file buffer missing message: %q", fileBuf.String())
	}
	if strings.Contains(consoleBuf.String(), "test message 42") {
		t.Error("verbose=false should not write info to console")
	}
}

func TestLoggerDetailfVerbose(t *testing.T) {
	var fileBuf, consoleBuf bytes.Buffer
	red := redactor.New()
	log := NewLogger(&fileBuf, &consoleBuf, red, true)
	log.Detailf("detail message")
	if !strings.Contains(fileBuf.String(), "detail message") {
		t.Errorf("file buffer missing detail: %q", fileBuf.String())
	}
	if !strings.Contains(consoleBuf.String(), "detail message") {
		t.Error("verbose=true should write detail to console")
	}
}

func TestLoggerDetailfNotVerbose(t *testing.T) {
	var fileBuf, consoleBuf bytes.Buffer
	red := redactor.New()
	log := NewLogger(&fileBuf, &consoleBuf, red, false)
	log.Detailf("hidden detail")
	if !strings.Contains(fileBuf.String(), "hidden detail") {
		t.Errorf("file buffer should always receive detail: %q", fileBuf.String())
	}
	if strings.Contains(consoleBuf.String(), "hidden detail") {
		t.Error("verbose=false should not write detail to console")
	}
}

func TestLoggerProgressfAlwaysConsole(t *testing.T) {
	var fileBuf, consoleBuf bytes.Buffer
	red := redactor.New()
	// verbose=false: Progressf should STILL write to console
	log := NewLogger(&fileBuf, &consoleBuf, red, false)
	log.Progressf("step %d of %d", 1, 3)
	if !strings.Contains(consoleBuf.String(), "step 1 of 3") {
		t.Errorf("Progressf should always write to console: %q", consoleBuf.String())
	}
	if !strings.Contains(fileBuf.String(), "step 1 of 3") {
		t.Errorf("Progressf should always write to file: %q", fileBuf.String())
	}
}
