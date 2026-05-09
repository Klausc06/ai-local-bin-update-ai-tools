package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"update-ai-tools/internal/redactor"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
	StatusWarning Status = "warning"
	StatusInfo    Status = "info"
)

type Report struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Mode       string       `json:"mode"`
	OS         string       `json:"os"`
	Arch       string       `json:"arch"`
	Home       string       `json:"home"`
	LogPath    string       `json:"log_path"`
	BackupDir  string       `json:"backup_dir,omitempty"`
	Summary    Summary      `json:"summary"`
	Inventory  []Item       `json:"inventory"`
	Risks      []Risk       `json:"risks"`
	Results    []TaskResult `json:"results"`
}

type Summary struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Warning int `json:"warning"`
	Info    int `json:"info"`
}

type Item struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Path     string `json:"path,omitempty"`
}

type Risk struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Level    string `json:"level"`
	Reason   string `json:"reason"`
	Path     string `json:"path,omitempty"`
}

type TaskResult struct {
	Name         string        `json:"name"`
	Provider     string        `json:"provider"`
	Status       Status        `json:"status"`
	Summary      string        `json:"summary"`
	Command      []string      `json:"command,omitempty"`
	ResolvedPath string        `json:"resolved_path,omitempty"`
	Duration     time.Duration `json:"duration"`
	Output       string        `json:"output,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type Logger struct {
	file    io.Writer
	console io.Writer
	red     redactor.Redactor
	verbose bool
}

func NewLogger(file, console io.Writer, red redactor.Redactor, verbose bool) *Logger {
	return &Logger{file: file, console: console, red: red, verbose: verbose}
}

func (l *Logger) Infof(format string, args ...any) {
	l.write("INFO", fmt.Sprintf(format, args...), l.verbose)
}

func (l *Logger) Detailf(format string, args ...any) {
	l.write("DETAIL", fmt.Sprintf(format, args...), l.verbose)
}

func (l *Logger) Progressf(format string, args ...any) {
	l.write("INFO", fmt.Sprintf(format, args...), false)
}

func (l *Logger) ProgressBar(step, total int, label string) {
	barWidth := 20
	filled := (step * barWidth) / total
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	line := fmt.Sprintf("\r  [%s] %d/%d %s", bar, step, total, label)
	fmt.Fprint(l.console, line)
	l.write("INFO", fmt.Sprintf("[%d/%d] %s", step, total, label), false)
}

func (l *Logger) ProgressDone() {
	fmt.Fprint(l.console, "\n\n")
}

func (l *Logger) write(level, msg string, toConsole bool) {
	line := l.red.Redact(fmt.Sprintf("%s %s %s\n", time.Now().Format(time.RFC3339), level, msg))
	if _, err := l.file.Write([]byte(line)); err != nil {
		fmt.Fprintf(l.console, "WARNING: log write failed: %v\n", err)
	}
	if toConsole {
		_, _ = l.console.Write([]byte(strings.TrimRight(line, "\n") + "\n"))
	}
}

func Summarize(results []TaskResult) Summary {
	var s Summary
	for _, r := range results {
		switch r.Status {
		case StatusSuccess:
			s.Success++
		case StatusFailed:
			s.Failed++
		case StatusSkipped:
			s.Skipped++
		case StatusWarning:
			s.Warning++
		default:
			s.Info++
		}
	}
	return s
}

func DeduplicateRisks(risks []Risk) []Risk {
	seen := map[string]bool{}
	out := make([]Risk, 0, len(risks))
	for _, risk := range risks {
		key := risk.Name + "\x00" + risk.Level + "\x00" + risk.Reason + "\x00" + risk.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, risk)
	}
	return out
}
