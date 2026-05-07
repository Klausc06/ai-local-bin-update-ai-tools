package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"update-ai-tools/internal/backup"
	"update-ai-tools/internal/platform"
	"update-ai-tools/internal/provider"
	"update-ai-tools/internal/redactor"
	"update-ai-tools/internal/report"
	"update-ai-tools/internal/runner"
)

const version = "0.1.0"

type options struct {
	check   bool
	dryRun  bool
	version bool
	jsonOut bool
	verbose bool
	home    string
	only    stringSet
	skip    stringSet
}

type stringSet map[string]bool

func Run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.version {
		fmt.Fprintf(os.Stdout, "update-ai-tools %s\n", version)
		return nil
	}

	started := time.Now()
	profile := platform.Detect(opts.home)
	red := redactor.New()
	logPath := logPath(profile.Home, started)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	console := io.Writer(os.Stdout)
	if opts.jsonOut {
		console = io.Discard
	}
	log := report.NewLogger(logFile, console, red, opts.verbose)
	cmdRunner := runner.New(red, log)

	registry := provider.DefaultRegistry(profile, cmdRunner)
	active := filterProviders(registry, opts.only, opts.skip)

	rep := report.Report{
		StartedAt: started,
		Mode:      modeName(opts.check, opts.dryRun),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		LogPath:   logPath,
		Home:      profile.Home,
	}

	log.Infof("update-ai-tools started mode=%s os=%s arch=%s", rep.Mode, rep.OS, rep.Arch)
	log.Infof("log file: %s", logPath)

	for _, p := range active {
		log.Infof("inventory: %s", p.Name())
		items, risks, results := p.Inventory()
		rep.Inventory = append(rep.Inventory, items...)
		rep.Risks = append(rep.Risks, risks...)
		rep.Results = append(rep.Results, results...)
	}
	rep.Risks = report.DeduplicateRisks(rep.Risks)

	if opts.dryRun {
		rep.Results = append(rep.Results, report.TaskResult{
			Name:     "backup-configs",
			Provider: "backup",
			Status:   report.StatusSkipped,
			Summary:  "dry-run: would back up configs before updating",
		})
		for _, p := range active {
			for _, task := range p.UpdateTasks() {
				rep.Results = append(rep.Results, report.TaskResult{
					Name:     task.Name,
					Provider: task.Provider,
					Status:   report.StatusSkipped,
					Summary:  "dry-run: would run " + strings.Join(task.Command, " "),
					Command:  task.Command,
				})
			}
		}
	} else if !opts.check {
		backupDir, result := backup.Configs(profile, red, log)
		rep.BackupDir = backupDir
		rep.Results = append(rep.Results, result)
		for _, p := range active {
			for _, task := range p.UpdateTasks() {
				log.Infof("update: %s", task.Name)
				rep.Results = append(rep.Results, cmdRunner.RunTask(task))
			}
		}
		for _, p := range active {
			rep.Results = append(rep.Results, p.PostUpdateChecks()...)
		}
	}

	rep.FinishedAt = time.Now()
	rep.Summary = report.Summarize(rep.Results)

	if opts.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		printHuman(os.Stdout, rep, red)
	}

	writeJSONReport(logFile, rep, red)
	return nil
}

func parseArgs(args []string) (options, error) {
	fs := flag.NewFlagSet("update-ai-tools", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := options{}
	fs.BoolVar(&opts.check, "check", false, "inventory only; do not update or back up configs")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "inventory and show planned update commands without backup or updates")
	fs.BoolVar(&opts.version, "version", false, "print version and exit")
	fs.BoolVar(&opts.jsonOut, "json", false, "print machine-readable JSON report")
	fs.BoolVar(&opts.verbose, "verbose", false, "print command details to terminal")
	home := fs.String("home", "", "override home directory for testing")
	only := fs.String("only", "", "comma-separated provider names to run")
	skip := fs.String("skip", "", "comma-separated provider names to skip")
	if err := fs.Parse(args); err != nil {
		return opts, usageError()
	}
	if fs.NArg() != 0 {
		return opts, usageError()
	}
	if opts.check && opts.dryRun {
		return opts, fmt.Errorf("--check and --dry-run are mutually exclusive")
	}
	opts.home = *home
	opts.only = parseSet(*only)
	opts.skip = parseSet(*skip)
	return opts, nil
}

func usageError() error {
	return fmt.Errorf(`Usage: update-ai-tools [--check|--dry-run] [--json] [--verbose] [--only names] [--skip names]

Examples:
  update-ai-tools --check
  update-ai-tools --dry-run
  update-ai-tools --version
  update-ai-tools
  update-ai-tools --check --json
`)
}

func parseSet(raw string) stringSet {
	out := stringSet{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.ToLower(strings.TrimSpace(part))
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func filterProviders(all []provider.Provider, only, skip stringSet) []provider.Provider {
	var out []provider.Provider
	for _, p := range all {
		name := strings.ToLower(p.Name())
		if len(only) > 0 && !only[name] {
			continue
		}
		if skip[name] {
			continue
		}
		out = append(out, p)
	}
	return out
}

func logPath(home string, t time.Time) string {
	return filepath.Join(home, ".codex", "log", "update-ai-tools", t.Format("20060102-150405")+".log")
}

func modeName(check, dryRun bool) string {
	if check {
		return "check"
	}
	if dryRun {
		return "dry-run"
	}
	return "update"
}

func printHuman(w io.Writer, rep report.Report, red redactor.Redactor) {
	fmt.Fprintf(w, "\nupdate-ai-tools %s summary\n", rep.Mode)
	fmt.Fprintf(w, "Log: %s\n", red.Redact(rep.LogPath))
	if rep.BackupDir != "" {
		fmt.Fprintf(w, "Backup: %s\n", red.Redact(rep.BackupDir))
	}
	fmt.Fprintf(w, "\nResults: success=%d failed=%d skipped=%d warning=%d info=%d\n",
		rep.Summary.Success, rep.Summary.Failed, rep.Summary.Skipped, rep.Summary.Warning, rep.Summary.Info)

	names := make([]string, 0, len(rep.Results))
	for _, r := range rep.Results {
		names = append(names, fmt.Sprintf("%-18s %-8s %s", r.Name, r.Status, r.Summary))
	}
	sort.Strings(names)
	for _, line := range names {
		fmt.Fprintln(w, red.Redact(line))
	}

	warnings := warningResults(rep.Results)
	if len(warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, warning := range warnings {
			fmt.Fprintf(w, "- %s: %s\n", warning.Name, red.Redact(warning.Summary))
		}
	}

	if len(rep.Risks) > 0 {
		fmt.Fprintln(w, "\nManual review:")
		for _, risk := range rep.Risks {
			fmt.Fprintf(w, "- [%s] %s: %s\n", risk.Level, risk.Name, red.Redact(risk.Reason))
		}
	}
	fmt.Fprintln(w)
}

func warningResults(results []report.TaskResult) []report.TaskResult {
	var warnings []report.TaskResult
	for _, result := range results {
		if result.Status == report.StatusWarning {
			warnings = append(warnings, result)
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].Name < warnings[j].Name
	})
	return warnings
}

func writeJSONReport(w io.Writer, rep report.Report, red redactor.Redactor) {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("\nJSON report\n"))
	_, _ = w.Write([]byte(red.Redact(string(data))))
	_, _ = w.Write([]byte("\n"))
}
