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
	if task.SkipIfMissing != "" {
		if _, err := exec.LookPath(task.SkipIfMissing); err != nil {
			res.Status = report.StatusSkipped
			res.Summary = task.SkipIfMissing + " not found"
			res.Duration = time.Since(start)
			return res
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
	res.Summary = firstSignificantLine(res.Output, "ok")
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

func firstSignificantLine(s, fallback string) string {
	s = strings.TrimSpace(s)
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

func looksLikeHealthWarning(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "failed to connect") || strings.Contains(lower, "✗")
}

func shouldTryFallback(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "unknown command") ||
		strings.Contains(lower, "unrecognized command") ||
		strings.Contains(lower, "unrecognized subcommand") ||
		strings.Contains(lower, "not found")
}
