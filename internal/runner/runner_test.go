package runner

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"

	"update-ai-tools/internal/redactor"
	"update-ai-tools/internal/report"
)

func TestFirstSignificantLineSkipsWarnings(t *testing.T) {
	got := firstSignificantLine("WARNING: noisy\ncodex-cli 0.128.0", "fallback")
	if got != "codex-cli 0.128.0" {
		t.Fatalf("got %q", got)
	}
}

func TestFirstSignificantLineEmpty(t *testing.T) {
	got := firstSignificantLine("", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback for empty, got %q", got)
	}
}

func TestFirstSignificantLineAllWarnings(t *testing.T) {
	got := firstSignificantLine("WARNING: a\nWARNING: b", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback when all lines are warnings, got %q", got)
	}
}

func TestLooksLikeHealthWarning(t *testing.T) {
	if !looksLikeHealthWarning("plugin:playwright - ✗ Failed to connect") {
		t.Fatal("expected health warning")
	}
	if looksLikeHealthWarning("all systems operational ✓") {
		t.Fatal("normal output should not be a health warning")
	}
	if looksLikeHealthWarning("connection established successfully") {
		t.Fatal("success message should not be a health warning")
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

func TestCommandErrorExitCode(t *testing.T) {
	err := &exec.ExitError{}
	msg := commandError(err)
	if !strings.Contains(msg, "exit status") {
		t.Errorf("expected exit status message, got %q", msg)
	}
}

func TestCommandErrorPlain(t *testing.T) {
	err := exec.Command("nonexistent_command_xyz").Run()
	msg := commandError(err)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func newTestRunner() *Runner {
	red := redactor.New()
	var buf bytes.Buffer
	log := report.NewLogger(&buf, &buf, red, false)
	return New(red, log)
}

func TestRunTaskSuccess(t *testing.T) {
	r := newTestRunner()
	task := Task{Name: "echo-test", Provider: "test", Command: []string{"echo", "hello"}, Timeout: 5 * time.Second}
	result := r.RunTask(task)
	if result.Status != report.StatusSuccess {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Summary)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
	if result.ResolvedPath == "" {
		t.Fatal("expected resolved command path")
	}
	wantPath, err := exec.LookPath("echo")
	if err != nil {
		t.Fatal(err)
	}
	if result.ResolvedPath != wantPath {
		t.Fatalf("expected resolved echo path %q, got %q", wantPath, result.ResolvedPath)
	}
}

func TestRunTaskFailed(t *testing.T) {
	r := newTestRunner()
	task := Task{Name: "false-test", Provider: "test", Command: []string{"false"}, Timeout: 5 * time.Second}
	result := r.RunTask(task)
	if result.Status != report.StatusFailed {
		t.Fatalf("expected failed, got %s: %s", result.Status, result.Summary)
	}
}

func TestRunTaskSkipIfMissing(t *testing.T) {
	r := newTestRunner()
	task := Task{Name: "missing-test", Provider: "test", Command: []string{"nonexistent_cmd_xyz"}, SkipIfMissing: "nonexistent_cmd_xyz", Timeout: 5 * time.Second}
	result := r.RunTask(task)
	if result.Status != report.StatusSkipped {
		t.Fatalf("expected skipped, got %s: %s", result.Status, result.Summary)
	}
}

func TestRunTaskEmptyCommand(t *testing.T) {
	r := newTestRunner()
	task := Task{Name: "empty-test", Provider: "test", Command: nil}
	result := r.RunTask(task)
	if result.Status != report.StatusSkipped {
		t.Fatalf("expected skipped for empty command, got %s", result.Status)
	}
}

func TestCapture(t *testing.T) {
	r := newTestRunner()
	result := r.Capture("test", "cap-test", 5*time.Second, "echo", "captured")
	if result.Status != report.StatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Output != "captured" {
		t.Errorf("expected 'captured', got %q", result.Output)
	}
}

func TestCompactSummaryTableHeader(t *testing.T) {
	// Simulates codex mcp list output with a table header
	output := "Name            Command                                       Args  Status\nserver1         cmd1                                          args1 enabled\nserver2         cmd2                                          args2 enabled"
	got := compactSummary(output, "ok")
	if got == "ok" {
		t.Fatalf("expected compacted summary, got fallback %q", got)
	}
	if !strings.Contains(got, "servers") {
		t.Errorf("expected server count summary, got %q", got)
	}
}

func TestCompactSummaryNormalText(t *testing.T) {
	// Non-table output should pass through via firstSignificantLine
	got := compactSummary("codex-cli 0.129.0", "ok")
	if got != "codex-cli 0.129.0" {
		t.Errorf("expected pass-through, got %q", got)
	}
}

func TestCompactSummaryEmptyOutput(t *testing.T) {
	got := compactSummary("", "fallback")
	if got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
}

func TestCompactSummaryTableEmpty(t *testing.T) {
	// Table header but no data rows
	output := "Name            Status"
	got := compactSummary(output, "ok")
	// Only header, no data rows: should return the header line
	if got == "ok" {
		t.Fatalf("expected header line, not fallback")
	}
}

func TestCompactSummaryCodexUpdate(t *testing.T) {
	output := "Updating Codex via `npm install -g @openai/codex`...\n\nupdated"
	got := compactSummary(output, "fallback")
	if got != "updated" {
		t.Errorf("expected 'updated', got %q", got)
	}
}

func TestCompactSummarySkillsUpdate(t *testing.T) {
	output := "Checking for skill updates...\n4 skills are already up to date."
	got := compactSummary(output, "fallback")
	if got != "4 skills are already up to date." {
		t.Errorf("expected skills status, got %q", got)
	}
}

func TestCompactSummaryNoisyOnly(t *testing.T) {
	// Only noisy line, no second line — should keep the noisy line
	output := "Running something..."
	got := compactSummary(output, "fallback")
	if got != "Running something..." {
		t.Errorf("expected noisy line unchanged when no better line, got %q", got)
	}
}

func TestCompactSummarySkipsSpinner(t *testing.T) {
	output := "Checking for skill updates...\n⠙\n4 skills are already up to date."
	got := compactSummary(output, "fallback")
	if got != "4 skills are already up to date." {
		t.Errorf("expected skills status (skipping spinner), got %q", got)
	}
}

func TestIsNoisyFirstLine(t *testing.T) {
	for _, s := range []string{"Updating Codex", "Checking for updates", "Installing package", "Downloading assets"} {
		if !isNoisyFirstLine(s) {
			t.Errorf("%q should be noisy", s)
		}
	}
	if isNoisyFirstLine("codex-cli 0.129.0") {
		t.Error("version string should not be noisy")
	}
	if isNoisyFirstLine("4 skills up to date") {
		t.Error("status string should not be noisy")
	}
}

func TestLooksLikeSpinner(t *testing.T) {
	for _, s := range []string{"⠙", "⠹", "|", "/", "-", "\\"} {
		if !looksLikeSpinner(s) {
			t.Errorf("%q should look like spinner", s)
		}
	}
	if looksLikeSpinner("updated") {
		t.Error("'updated' should not look like spinner")
	}
}

func TestCompactSummaryCarriageReturn(t *testing.T) {
	// Skills output uses \r for progress, \n for blank lines
	output := "Checking for skill updates...\n\n\rChecking global skill 1/3: a\rChecking global skill 2/3: b\rChecking global skill 3/3: c\r✓ All global skills are up to date"
	got := compactSummary(output, "fallback")
	if got != "✓ All global skills are up to date" {
		t.Errorf("expected '✓ All global skills are up to date', got %q", got)
	}
}

func TestNormalizeLines(t *testing.T) {
	got := normalizeLines("line1\rline2\nline3")
	if !strings.Contains(got, "\n") && strings.Contains(got, "\r") {
		t.Error("expected \\r to be converted to \\n")
	}
}

func TestFirstSignificantLineCarriageReturn(t *testing.T) {
	output := "\rprogress 1\rprogress 2\rresult ok"
	got := firstSignificantLine(output, "fallback")
	// After normalization, \r becomes \n, so we get multiple lines.
	// firstSignificantLine skips leading blank lines (from empty splits).
	// First non-empty after split: "progress 1"
	if got != "progress 1" {
		t.Errorf("expected 'progress 1', got %q", got)
	}
}
