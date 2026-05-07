package app

import (
	"bufio"
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

var version = "0.1.0-dev"

type options struct {
	check   bool
	dryRun  bool
	menu    bool
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
	if opts.menu {
		if !isTerminal() {
			return fmt.Errorf("--menu requires an interactive terminal")
		}
		menuArgs, err := interactiveSelect()
		if err != nil {
			return err
		}
		return Run(menuArgs)
	}
	if opts.version {
		fmt.Fprintf(os.Stdout, "update-ai-tools %s\n", version)
		return nil
	}

	started := time.Now()
	profile := platform.Detect(opts.home)
	if profile.Home == "" {
		return fmt.Errorf("cannot determine home directory; set --home")
	}
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
	fs.BoolVar(&opts.menu, "menu", false, "show an interactive action menu")
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
	return fmt.Errorf("usage: update-ai-tools [--check|--dry-run|--menu] [--json] [--verbose] [--only names] [--skip names]\n\n" +
		"Examples:\n" +
		"  update-ai-tools --check\n" +
		"  update-ai-tools --dry-run\n" +
		"  update-ai-tools --menu\n" +
		"  update-ai-tools --version\n" +
		"  update-ai-tools\n" +
		"  update-ai-tools --check --json")
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
	fmt.Fprintf(w, "\nupdate-ai-tools  %s  summary\n", rep.Mode)
	fmt.Fprintf(w, "\n  Log      %s\n", red.Redact(rep.LogPath))
	if rep.BackupDir != "" {
		fmt.Fprintf(w, "  Backup   %s\n", red.Redact(rep.BackupDir))
	}

	fmt.Fprintf(w, "\n  success %d  ·  failed %d  ·  skipped %d",
		rep.Summary.Success, rep.Summary.Failed, rep.Summary.Skipped)
	if rep.Summary.Warning > 0 || rep.Summary.Info > 0 {
		fmt.Fprintf(w, "  ·  warning %d", rep.Summary.Warning)
	}
	fmt.Fprintln(w)

	if len(rep.Results) == 0 {
		return
	}

	// Calculate column widths.
	nameW := 12
	for _, r := range rep.Results {
		if len(r.Name) > nameW {
			nameW = len(r.Name)
		}
	}
	if nameW > 28 {
		nameW = 28
	}

	fmt.Fprintln(w)
	nameHdr := "NAME" + strings.Repeat(" ", nameW-4)
	fmt.Fprintf(w, "  %s  STATUS    SUMMARY\n", nameHdr)

	sort.Slice(rep.Results, func(i, j int) bool {
		if rep.Results[i].Provider != rep.Results[j].Provider {
			return rep.Results[i].Provider < rep.Results[j].Provider
		}
		return rep.Results[i].Name < rep.Results[j].Name
	})

	for _, r := range rep.Results {
		name := r.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		summary := strings.TrimRight(r.Summary, "\n")
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}
		line := fmt.Sprintf("  %-*s  %-8s  %s", nameW, name, r.Status, summary)
		fmt.Fprintln(w, red.Redact(line))
	}

	warnings := warningResults(rep.Results)
	if len(warnings) > 0 {
		fmt.Fprintf(w, "\n  %d warning(s)\n", len(warnings))
		for _, warning := range warnings {
			summary := strings.TrimRight(warning.Summary, "\n")
			if len(summary) > 60 {
				summary = summary[:57] + "..."
			}
			fmt.Fprintf(w, "  %-*s  %s\n", nameW, warning.Name, red.Redact(summary))
		}
	}

	risks := report.DeduplicateRisks(rep.Risks)
	if len(risks) > 0 {
		fmt.Fprintf(w, "\n  %d risk(s)\n", len(risks))
		sort.Slice(risks, func(i, j int) bool {
			if risks[i].Level != risks[j].Level {
				return risks[i].Level < risks[j].Level
			}
			return risks[i].Reason < risks[j].Reason
		})
		for _, risk := range risks {
			detail := risk.Name
			if risk.Path != "" {
				detail = risk.Path
			}
			fmt.Fprintf(w, "  [%s]  %s  — %s\n", risk.Level, red.Redact(detail), risk.Reason)
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
		fmt.Fprintf(os.Stderr, "update-ai-tools: failed to marshal JSON report: %v\n", err)
		return
	}
	_, _ = w.Write([]byte("\nJSON report\n"))
	_, _ = w.Write([]byte(red.Redact(string(data))))
	_, _ = w.Write([]byte("\n"))
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func interactiveSelect() ([]string, error) {
	fmt.Println()
	fmt.Println("update-ai-tools")
	fmt.Println()
	fmt.Println("Choose an action:")
	fmt.Println("  [1] Check        — inventory only, no changes")
	fmt.Println("  [2] Dry run      — show planned update commands")
	fmt.Println("  [3] Update       — backup configs and run updates")
	fmt.Println("  [4] Check (JSON) — inventory only, JSON output")
	fmt.Println("  [5] Version      — print version and exit")
	fmt.Println()
	fmt.Print("Enter 1-5: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return nil, fmt.Errorf("read menu selection: %w", err)
	}
	switch strings.TrimSpace(line) {
	case "2":
		return []string{"--dry-run"}, nil
	case "3":
		return []string{}, nil
	case "4":
		return []string{"--check", "--json"}, nil
	case "5":
		return []string{"--version"}, nil
	default:
		return []string{"--check"}, nil
	}
}
