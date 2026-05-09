package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"update-ai-tools/internal/redactor"
	"update-ai-tools/internal/report"
)

type Task struct {
	Name             string
	Provider         string
	Command          []string
	FallbackCommands [][]string
	Timeout          time.Duration
	SkipIfMissing    string
}

type Runner struct {
	red redactor.Redactor
	log *report.Logger
}

func New(red redactor.Redactor, log *report.Logger) *Runner {
	return &Runner{red: red, log: log}
}

func (r *Runner) RunTask(task Task) report.TaskResult {
	start := time.Now()
	res := report.TaskResult{Name: task.Name, Provider: task.Provider, Command: task.Command}
	if len(task.Command) == 0 {
		res.Status = report.StatusSkipped
		res.Summary = "no command configured"
		return res
	}
	if path, err := exec.LookPath(task.Command[0]); err == nil {
		res.ResolvedPath = path
	}
	if task.SkipIfMissing != "" {
		path, err := exec.LookPath(task.SkipIfMissing)
		if err != nil {
			res.Status = report.StatusSkipped
			res.Summary = task.SkipIfMissing + " not found"
			res.Duration = time.Since(start)
			return res
		}
		if task.SkipIfMissing == task.Command[0] {
			res.ResolvedPath = path
		}
	}
	timeout := task.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, task.Command[0], task.Command[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	res.Duration = time.Since(start)
	res.Output = strings.TrimSpace(r.red.Redact(out.String()))
	if ctx.Err() != nil {
		res.Status = report.StatusFailed
		res.Summary = "timed out"
		res.Error = ctx.Err().Error()
		r.log.Detailf("%s timed out: %s", task.Name, res.Output)
		return res
	}
	if err != nil {
		if shouldTryFallback(res.Output) {
			for _, fallback := range task.FallbackCommands {
				if len(fallback) == 0 {
					continue
				}
				next := task
				next.Command = fallback
				next.FallbackCommands = nil
				next.SkipIfMissing = fallback[0]
				return r.RunTask(next)
			}
		}
		res.Status = report.StatusFailed
		res.Summary = "command failed"
		res.Error = commandError(err)
		r.log.Detailf("%s failed: %s", task.Name, res.Output)
		return res
	}
	if looksLikeHealthWarning(res.Output) {
		res.Status = report.StatusWarning
		res.Summary = firstSignificantLine(res.Output, "completed with warnings")
		return res
	}
	res.Status = report.StatusSuccess
	res.Summary = compactSummary(res.Output, "ok")
	r.log.Detailf("%s output: %s", task.Name, res.Output)
	return res
}

func (r *Runner) Capture(provider, name string, timeout time.Duration, command ...string) report.TaskResult {
	task := Task{Name: name, Provider: provider, Command: command, Timeout: timeout}
	if len(command) > 0 {
		task.SkipIfMissing = command[0]
	}
	return r.RunTask(task)
}

func commandError(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Sprintf("exit status %d", exit.ExitCode())
	}
	return err.Error()
}

func compactSummary(output, fallback string) string {
	first := firstSignificantLine(output, fallback)

	// MCP table header (Name ... Status columns) → "N servers".
	if strings.Contains(first, "Name") && strings.Contains(first, "Status") {
		lines := strings.Split(normalizeLines(output), "\n")
		count := 0
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
		if count > 0 {
			return fmt.Sprintf("%d servers", count)
		}
		return first
	}

	// Noisy progress lines → try to find the last meaningful line.
	if isNoisyFirstLine(first) {
		if better := lastSignificantLine(output); better != "" {
			return better
		}
	}

	return first
}

func isNoisyFirstLine(s string) bool {
	noisy := []string{
		"Updating ", "Checking ", "Installing ", "Downloading ",
		"Starting ", "Running ", "Fetching ", "Building ",
	}
	for _, prefix := range noisy {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func lastSignificantLine(s string) string {
	lines := strings.Split(normalizeLines(s), "\n")
	// Walk backwards to find the last meaningful, non-spinner line.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "WARNING:") {
			continue
		}
		if looksLikeSpinner(line) {
			continue
		}
		return line
	}
	return ""
}

func looksLikeSpinner(s string) bool {
	// Spinner frames: ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏ ⠋ or | / - \
	spinners := []string{"⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", "⠋", "|", "/", "-", "\\"}
	for _, sp := range spinners {
		if strings.HasPrefix(strings.TrimSpace(s), sp) {
			return true
		}
	}
	return false
}

func firstSignificantLine(s, fallback string) string {
	s = normalizeLines(s)
	if s == "" {
		return fallback
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "WARNING:") {
			continue
		}
		return line
	}
	return fallback
}

func normalizeLines(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\r", "\n")
}

func looksLikeHealthWarning(s string) bool {
	return strings.Contains(strings.ToLower(s), "failed to connect")
}

func shouldTryFallback(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "unknown command") ||
		strings.Contains(lower, "unrecognized command") ||
		strings.Contains(lower, "unrecognized subcommand") ||
		strings.Contains(lower, "not found")
}
