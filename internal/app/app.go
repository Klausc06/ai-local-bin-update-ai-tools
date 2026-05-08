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

type action string

const (
	actionCheck   action = "check"
	actionDryRun  action = "dry-run"
	actionMenu    action = "menu"
	actionUpdate  action = "update"
	actionVersion action = "version"
)

type options struct {
	action  action
	check   bool
	dryRun  bool
	menu    bool
	update  bool
	version bool
	force   bool
	jsonOut bool
	verbose bool
	home    string
	only    stringSet
	skip    stringSet
}

type stringSet map[string]bool

func Run(args []string) error {
	args = defaultArgs(args)
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.menu {
		if !isTerminal() {
			return fmt.Errorf("--menu requires an interactive terminal; use --check or --json for non-interactive environments")
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
	logPath, logFile, err := createLogFile(profile.Home, started)
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
	if err := validateProviderFilters(registry, opts.only, opts.skip); err != nil {
		return err
	}
	active := filterProviders(registry, opts.only, opts.skip)

	rep := report.Report{
		StartedAt: started,
		Mode:      modeName(opts.action),
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

	var runErr error
	selectedUpdateTasks := map[string]bool{}
	if opts.action == actionDryRun {
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
	} else if opts.action == actionUpdate {
		backupDir, result := backup.Configs(profile, red, log)
		rep.BackupDir = backupDir
		rep.Results = append(rep.Results, result)
		canContinueAfterBackup := result.Status == report.StatusSuccess ||
			(opts.force && result.Status == report.StatusWarning)
		if !canContinueAfterBackup {
			msg := "backup did not complete cleanly; updates skipped (use --force to override)"
			log.Infof(msg)
			rep.Results = append(rep.Results, report.TaskResult{
				Name:     "updates-skipped",
				Provider: "update",
				Status:   report.StatusSkipped,
				Summary:  msg,
			})
			runErr = fmt.Errorf("backup did not complete cleanly; updates skipped")
		} else {
			if result.Status != report.StatusSuccess && opts.force {
				log.Infof("force enabled; continuing after backup warning")
			}
			for _, p := range active {
				for _, task := range p.UpdateTasks() {
					selectedUpdateTasks[task.Name] = true
					log.Infof("update: %s", task.Name)
					rep.Results = append(rep.Results, cmdRunner.RunTask(task))
				}
			}
			for _, p := range active {
				rep.Results = append(rep.Results, p.PostUpdateChecks()...)
			}
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
		printHuman(os.Stdout, rep, red, opts.verbose)
	}

	writeJSONReport(logFile, rep, red)
	if runErr != nil {
		return runErr
	}
	if opts.action == actionUpdate {
		if names := taskNamesWithStatus(rep.Results, selectedUpdateTasks, report.StatusSkipped); len(names) > 0 {
			return fmt.Errorf("update completed with %d skipped update task(s): %s", len(names), strings.Join(names, ", "))
		}
	}
	if opts.action == actionUpdate && rep.Summary.Failed > 0 {
		return fmt.Errorf("update completed with %d failed task(s): %s", rep.Summary.Failed, strings.Join(failedTaskNames(rep.Results), ", "))
	}
	return nil
}

func parseArgs(args []string) (options, error) {
	fs := flag.NewFlagSet("update-ai-tools", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := options{}
	fs.BoolVar(&opts.check, "check", false, "inventory only; do not update or back up configs")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "inventory and show planned update commands without backup or updates")
	fs.BoolVar(&opts.menu, "menu", false, "show an interactive action menu")
	fs.BoolVar(&opts.update, "update", false, "back up configs and run safe updates")
	fs.BoolVar(&opts.version, "version", false, "print version and exit")
	fs.BoolVar(&opts.force, "force", false, "continue update even if backup reports warnings or failures")
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
	if countActions(opts.check, opts.dryRun, opts.menu, opts.update, opts.version) > 1 {
		return opts, fmt.Errorf("--check, --dry-run, --menu, --update, and --version are mutually exclusive")
	}
	opts.home = *home
	opts.only = parseSet(*only)
	opts.skip = parseSet(*skip)
	switch {
	case opts.check:
		opts.action = actionCheck
	case opts.dryRun:
		opts.action = actionDryRun
	case opts.menu:
		opts.action = actionMenu
	case opts.update:
		opts.action = actionUpdate
	case opts.version:
		opts.action = actionVersion
	default:
		opts.action = actionCheck
		opts.check = true
	}
	return opts, nil
}

func usageError() error {
	return fmt.Errorf("usage: update-ai-tools [--check|--dry-run|--menu|--update|--version] [--json] [--verbose] [--force] [--only names] [--skip names]\n\n" +
		"Examples:\n" +
		"  update-ai-tools\n" +
		"  update-ai-tools --check\n" +
		"  update-ai-tools --dry-run\n" +
		"  update-ai-tools --update\n" +
		"  update-ai-tools --version\n" +
		"  update-ai-tools --check --json")
}

func defaultArgs(args []string) []string {
	if len(args) == 0 {
		if isTerminal() {
			return []string{"--menu"}
		}
		return []string{"--check"}
	}
	return args
}

func countActions(actions ...bool) int {
	count := 0
	for _, action := range actions {
		if action {
			count++
		}
	}
	return count
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

func failedTaskNames(results []report.TaskResult) []string {
	var names []string
	for _, result := range results {
		if result.Status == report.StatusFailed {
			names = append(names, result.Name)
		}
	}
	sort.Strings(names)
	return names
}

func taskNamesWithStatus(results []report.TaskResult, taskNames map[string]bool, status report.Status) []string {
	var names []string
	for _, result := range results {
		if taskNames[result.Name] && result.Status == status {
			names = append(names, result.Name)
		}
	}
	sort.Strings(names)
	return names
}

func validateProviderFilters(all []provider.Provider, only, skip stringSet) error {
	valid := map[string]bool{}
	var names []string
	for _, p := range all {
		name := strings.ToLower(p.Name())
		valid[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	for flagName, set := range map[string]stringSet{"--only": only, "--skip": skip} {
		for name := range set {
			if !valid[name] {
				return fmt.Errorf("%s contains unknown provider %q (valid: %s)", flagName, name, strings.Join(names, ", "))
			}
		}
	}
	return nil
}

func logPath(home string, t time.Time) string {
	return filepath.Join(home, ".codex", "log", "update-ai-tools", t.Format("20060102-150405")+".log")
}

func createLogFile(home string, t time.Time) (string, *os.File, error) {
	base := logPath(home, t)
	if err := os.MkdirAll(filepath.Dir(base), 0o700); err != nil {
		return base, nil, err
	}
	candidates := []string{base}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i <= 99; i++ {
		candidates = append(candidates, fmt.Sprintf("%s-%02d%s", stem, i, ext))
	}
	for _, candidate := range candidates {
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return candidate, f, nil
		}
		if os.IsExist(err) {
			continue
		}
		return candidate, nil, err
	}
	return base, nil, fmt.Errorf("all log filenames already exist for %s", filepath.Base(base))
}

func modeName(a action) string {
	if a == "" {
		return string(actionCheck)
	}
	return string(a)
}

func printHuman(w io.Writer, rep report.Report, red redactor.Redactor, verbose bool) {
	c := detectColors(w)

	fmt.Fprintln(w)
	c.header("  update-ai-tools  %s", rep.Mode)
	if rep.Mode == "dry-run" {
		fmt.Fprint(w, "  (no changes made)")
	}
	fmt.Fprintln(w)

	// ── results bar ────────────────────────────────────────────
	fmt.Fprintf(w, "  %s %d  %s %d  %s %d",
		c.ok("✓"), rep.Summary.Success,
		c.fail("✗"), rep.Summary.Failed,
		c.skip("●"), rep.Summary.Skipped)
	if rep.Summary.Warning > 0 {
		fmt.Fprintf(w, "  %s %d", c.warn("!"), rep.Summary.Warning)
	}
	// log path — dim, single line
	shortLog := rep.LogPath
	if home, _ := os.UserHomeDir(); home != "" {
		shortLog = strings.Replace(shortLog, home, "~", 1)
	}
	fmt.Fprintf(w, "\n  %s", c.dim(red.Redact(shortLog)))
	if rep.BackupDir != "" {
		fmt.Fprintf(w, "\n  %s", c.dim(red.Redact(strings.Replace(rep.BackupDir, os.Getenv("HOME"), "~", 1))))
	}

	actions, checks, support := splitResults(rep.Results)
	if verbose {
		printResultSection(w, "Checks", checks, red, c)
	}
	printResultSection(w, "Actions", actions, red, c)
	if verbose {
		printRisksSection(w, rep.Risks, red, c)
	} else {
		printRisksSectionBrief(w, rep.Risks, red, c)
	}
	printWarningsSection(w, warningResults(rep.Results), red, c)
	if verbose {
		printResultSection(w, "Details", support, red, c)
	}
	fmt.Fprintln(w)
}

func splitResults(results []report.TaskResult) (actions, checks, support []report.TaskResult) {
	for _, result := range results {
		switch {
		case result.Provider == "backup":
			support = append(support, result)
		case strings.HasSuffix(result.Name, "-version") ||
			strings.HasSuffix(result.Name, "-mcp-list") ||
			strings.HasSuffix(result.Name, "-mcp-list-after") ||
			strings.HasSuffix(result.Name, "-doctor") ||
			strings.HasSuffix(result.Name, "-doctor-after"):
			checks = append(checks, result)
		case strings.Contains(result.Name, "update"):
			actions = append(actions, result)
		default:
			support = append(support, result)
		}
	}
	sortResults(actions)
	sortResults(checks)
	sortResults(support)
	return actions, checks, support
}

func sortResults(results []report.TaskResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Provider != results[j].Provider {
			return results[i].Provider < results[j].Provider
		}
		return results[i].Name < results[j].Name
	})
}

func printResultSection(w io.Writer, title string, results []report.TaskResult, red redactor.Redactor, c colors) {
	if len(results) == 0 {
		return
	}
	fmt.Fprintln(w)
	c.section("  %s", title)
	for _, result := range results {
		marker, colorFn := statusGlyph(result.Status, c)
		summary := strings.TrimSpace(result.Summary)
		fmt.Fprintf(w, "\n    %s  %-26s  %s", colorFn(marker), result.Name, red.Redact(summary))
		if result.Status == report.StatusFailed && result.Error != "" {
			fmt.Fprintf(w, "  (%s)", result.Error)
		}
	}
	fmt.Fprintln(w)
}

func printWarningsSection(w io.Writer, warnings []report.TaskResult, red redactor.Redactor, c colors) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w)
	c.section("  Warnings")
	for _, wr := range warnings {
		fmt.Fprintf(w, "\n    %s  %-26s  %s", c.warn("!"), wr.Name, red.Redact(strings.TrimSpace(wr.Summary)))
	}
	fmt.Fprintln(w)
}

func printRisksSection(w io.Writer, risks []report.Risk, red redactor.Redactor, c colors) {
	risks = report.DeduplicateRisks(risks)
	if len(risks) == 0 {
		return
	}

	// split: actionable risks vs. informational (manual/sensitive are just FYI)
	var action, info []report.Risk
	for _, r := range risks {
		if r.Level == "manual" || r.Level == "sensitive" {
			info = append(info, r)
		} else {
			action = append(action, r)
		}
	}

	printRiskGroup(w, action, "Risks", c)
	printRiskGroup(w, info, "Advisory", c)
}

func printRisksSectionBrief(w io.Writer, risks []report.Risk, red redactor.Redactor, c colors) {
	risks = report.DeduplicateRisks(risks)
	if len(risks) == 0 {
		return
	}
	var action []report.Risk
	for _, r := range risks {
		if r.Level != "manual" && r.Level != "sensitive" {
			action = append(action, r)
		}
	}
	printRiskGroup(w, action, "Risks", c)
}

func printRiskGroup(w io.Writer, risks []report.Risk, title string, c colors) {
	if len(risks) == 0 {
		return
	}
	// sort by level then reason
	sort.Slice(risks, func(i, j int) bool {
		if risks[i].Level != risks[j].Level {
			return risks[i].Level < risks[j].Level
		}
		return risks[i].Reason < risks[j].Reason
	})

	fmt.Fprintln(w)
	c.section("  %s", title)
	for _, r := range risks {
		// show path (prettified) as the key detail
		detail := shortPath(r.Path)
		if detail == "" {
			detail = r.Name
		}
		levelFn := levelColor(r.Level, c)
		fmt.Fprintf(w, "\n    %s  %-42s  %s", levelFn(r.Level), detail, r.Reason)
	}
	fmt.Fprintln(w)
}

// ── color support ─────────────────────────────────────────────

type colors struct {
	ok, fail, skip, warn, dim func(string) string
	header, section           fmtFunc
}

type fmtFunc func(string, ...interface{}) (int, error)

func detectColors(w io.Writer) colors {
	f, ok := w.(*os.File)
	if !ok {
		return noopColors(w)
	}
	fi, err := f.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return noopColors(w)
	}
	return colors{
		ok:      green,
		fail:    red,
		skip:    yellow,
		warn:    yellow,
		dim:     dim,
		section: makeSectionFmt(w, "34"),
		header:  makeSectionFmt(w, "36"),
	}
}

func noopColors(w io.Writer) colors {
	noopStr := func(s string) string { return s }
	return colors{
		ok: noopStr, fail: noopStr, skip: noopStr, warn: noopStr, dim: noopStr,
		section: func(format string, args ...interface{}) (int, error) {
			return fmt.Fprintf(w, format, args...)
		},
		header: func(format string, args ...interface{}) (int, error) {
			return fmt.Fprintf(w, format, args...)
		},
	}
}

func makeSectionFmt(w io.Writer, code string) fmtFunc {
	return func(format string, args ...interface{}) (int, error) {
		return fmt.Fprintf(w, "\033[1;"+code+"m"+format+"\033[0m", args...)
	}
}

func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func dim(s string) string    { return "\033[2m" + s + "\033[0m" }

func statusGlyph(status report.Status, c colors) (string, func(string) string) {
	switch status {
	case report.StatusSuccess:
		return "✓", c.ok
	case report.StatusFailed:
		return "✗", c.fail
	case report.StatusSkipped:
		return "●", c.skip
	case report.StatusWarning:
		return "!", c.warn
	default:
		return "·", c.dim
	}
}

func levelColor(level string, c colors) func(string) string {
	switch level {
	case "high":
		return c.fail
	case "medium":
		return c.warn
	case "manual", "sensitive":
		return c.dim
	default:
		return c.dim
	}
}

func shortPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + strings.TrimPrefix(path, home)
	}
	if len(path) <= 48 {
		return path
	}
	return "..." + path[len(path)-45:]
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

var isTerminal = func() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	fo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fo.Mode() & os.ModeCharDevice) != 0
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
		return []string{"--update"}, nil
	case "4":
		return []string{"--check", "--json"}, nil
	case "5":
		return []string{"--version"}, nil
	default:
		return []string{"--check"}, nil
	}
}
