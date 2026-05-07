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
