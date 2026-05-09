package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"update-ai-tools/internal/provider"
	"update-ai-tools/internal/redactor"
	"update-ai-tools/internal/report"
	"update-ai-tools/internal/runner"
)

func TestParseSet(t *testing.T) {
	got := parseSet("codex, claude , ,OMX")
	if len(got) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(got), got)
	}
	for _, key := range []string{"codex", "claude", "omx"} {
		if !got[key] {
			t.Errorf("expected key %q", key)
		}
	}
}

func TestParseSetEmpty(t *testing.T) {
	got := parseSet("")
	if len(got) != 0 {
		t.Fatalf("expected empty set, got %d", len(got))
	}
}

func TestModeName(t *testing.T) {
	cases := []struct {
		action action
		want   string
	}{
		{"", "check"},
		{actionCheck, "check"},
		{actionDryRun, "dry-run"},
		{actionUpdate, "update"},
	}
	for _, c := range cases {
		got := modeName(c.action)
		if got != c.want {
			t.Errorf("modeName(%q) = %q, want %q", c.action, got, c.want)
		}
	}
}

func TestCreateLogFileUsesSuffixOnCollision(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 5, 7, 12, 34, 56, 0, time.UTC)
	firstPath, firstFile, err := createLogFile(home, now)
	if err != nil {
		t.Fatal(err)
	}
	defer firstFile.Close()
	secondPath, secondFile, err := createLogFile(home, now)
	if err != nil {
		t.Fatal(err)
	}
	defer secondFile.Close()
	if firstPath == secondPath {
		t.Fatalf("expected unique log paths, both were %q", firstPath)
	}
	if !strings.HasSuffix(secondPath, "-01.log") {
		t.Fatalf("expected suffixed log path, got %q", secondPath)
	}
}

func TestParseArgsDefaults(t *testing.T) {
	opts, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.action != actionCheck || !opts.check || opts.dryRun || opts.version || opts.jsonOut || opts.verbose {
		t.Fatalf("expected default check action, got %+v", opts)
	}
	if len(opts.only) != 0 || len(opts.skip) != 0 {
		t.Fatal("expected empty only/skip")
	}
}

func TestDefaultArgsUsesMenu(t *testing.T) {
	orig := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = orig }()

	got := defaultArgs(nil)
	if len(got) != 1 || got[0] != "--menu" {
		t.Fatalf("expected no-arg default to --menu, got %v", got)
	}
	original := []string{"--check"}
	got = defaultArgs(original)
	if len(got) != 1 || got[0] != "--check" {
		t.Fatalf("expected explicit args to be preserved, got %v", got)
	}
}

func TestParseArgsCheck(t *testing.T) {
	opts, err := parseArgs([]string{"--check"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.check || opts.dryRun {
		t.Fatal("expected check=true, dryRun=false")
	}
}

func TestParseArgsDryRun(t *testing.T) {
	opts, err := parseArgs([]string{"--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.check || !opts.dryRun {
		t.Fatal("expected dryRun=true, check=false")
	}
}

func TestParseArgsCheckAndDryRunExclusive(t *testing.T) {
	_, err := parseArgs([]string{"--check", "--dry-run"})
	if err == nil {
		t.Fatal("expected error for --check --dry-run together")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got: %v", err)
	}
}

func TestParseArgsVersion(t *testing.T) {
	opts, err := parseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.version {
		t.Fatal("expected version=true")
	}
}

func TestParseArgsMenu(t *testing.T) {
	opts, err := parseArgs([]string{"--menu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.menu {
		t.Fatal("expected menu=true")
	}
}

func TestParseArgsUpdate(t *testing.T) {
	opts, err := parseArgs([]string{"--update"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.update {
		t.Fatal("expected update=true")
	}
}

func TestParseArgsActionsExclusive(t *testing.T) {
	_, err := parseArgs([]string{"--menu", "--update"})
	if err == nil {
		t.Fatal("expected error for --menu --update together")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got: %v", err)
	}
}

func TestParseArgsJson(t *testing.T) {
	opts, err := parseArgs([]string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.jsonOut {
		t.Fatal("expected jsonOut=true")
	}
	if opts.action != actionCheck || !opts.check {
		t.Fatalf("expected non-action --json to default to check, got %+v", opts)
	}
}

func TestParseArgsNonActionFlagsDefaultToCheck(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "json", args: []string{"--json"}},
		{name: "only", args: []string{"--only", "skills"}},
		{name: "skip", args: []string{"--skip", "mcp"}},
		{name: "verbose", args: []string{"--verbose"}},
		{name: "combined", args: []string{"--json", "--verbose", "--only", "skills"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseArgs(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !opts.check {
				t.Fatalf("expected non-action args %v to default to check mode", tc.args)
			}
			if opts.dryRun || opts.menu || opts.update {
				t.Fatalf("expected non-action args %v not to select a mutating mode", tc.args)
			}
		})
	}
}

func TestParseArgsOnly(t *testing.T) {
	opts, err := parseArgs([]string{"--only", "codex,claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.only) != 2 || !opts.only["codex"] || !opts.only["claude"] {
		t.Errorf("expected only={codex,claude}, got %v", opts.only)
	}
}

func TestParseArgsSkip(t *testing.T) {
	opts, err := parseArgs([]string{"--skip", "skills"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.skip) != 1 || !opts.skip["skills"] {
		t.Errorf("expected skip={skills}, got %v", opts.skip)
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	_, err := parseArgs([]string{"--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseArgsExtraArgs(t *testing.T) {
	_, err := parseArgs([]string{"extra"})
	if err == nil {
		t.Fatal("expected error for extra positional args")
	}
}

func TestFilterProvidersOnly(t *testing.T) {
	all := []provider.Provider{
		stubProvider{name: "codex"},
		stubProvider{name: "claude"},
		stubProvider{name: "skills"},
	}
	only := stringSet{"codex": true}
	result := filterProviders(all, only, nil)
	if len(result) != 1 || result[0].Name() != "codex" {
		t.Errorf("expected [codex], got %v", providerNames(result))
	}
}

func TestFilterProvidersSkip(t *testing.T) {
	all := []provider.Provider{
		stubProvider{name: "codex"},
		stubProvider{name: "claude"},
		stubProvider{name: "skills"},
	}
	skip := stringSet{"skills": true}
	result := filterProviders(all, nil, skip)
	if len(result) != 2 {
		t.Errorf("expected 2 providers, got %d", len(result))
	}
	for _, p := range result {
		if p.Name() == "skills" {
			t.Error("skills should have been skipped")
		}
	}
}

func TestFilterProvidersOnlyAndSkip(t *testing.T) {
	all := []provider.Provider{
		stubProvider{name: "codex"},
		stubProvider{name: "claude"},
	}
	only := stringSet{"codex": true, "claude": true}
	skip := stringSet{"codex": true}
	result := filterProviders(all, only, skip)
	if len(result) != 1 || result[0].Name() != "claude" {
		t.Errorf("expected [claude], got %v", providerNames(result))
	}
}

func TestFilterProvidersCaseInsensitive(t *testing.T) {
	all := []provider.Provider{
		stubProvider{name: "Codex"},
	}
	only := stringSet{"codex": true}
	result := filterProviders(all, only, nil)
	if len(result) != 1 {
		t.Fatal("expected case-insensitive match")
	}
}

func TestValidateProviderFiltersRejectsUnknownOnly(t *testing.T) {
	all := []provider.Provider{
		stubProvider{name: "codex"},
		stubProvider{name: "skills"},
	}
	err := validateProviderFilters(all, stringSet{"codex": true, "ghost": true}, nil)
	if err == nil {
		t.Fatal("expected unknown --only provider to be rejected")
	}
	if !strings.Contains(err.Error(), "--only") || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected --only ghost error, got %v", err)
	}
}

func TestValidateProviderFiltersRejectsUnknownSkip(t *testing.T) {
	all := []provider.Provider{
		stubProvider{name: "codex"},
		stubProvider{name: "skills"},
	}
	err := validateProviderFilters(all, nil, stringSet{"ghost": true})
	if err == nil {
		t.Fatal("expected unknown --skip provider to be rejected")
	}
	if !strings.Contains(err.Error(), "--skip") || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected --skip ghost error, got %v", err)
	}
}

func TestWarningResults(t *testing.T) {
	results := []report.TaskResult{
		{Name: "a", Status: report.StatusSuccess},
		{Name: "b", Status: report.StatusWarning, Summary: "warn1"},
		{Name: "c", Status: report.StatusFailed},
		{Name: "d", Status: report.StatusWarning, Summary: "warn2"},
	}
	warnings := warningResults(results)
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(warnings))
	}
	if warnings[0].Name != "b" || warnings[1].Name != "d" {
		t.Errorf("unexpected warning order: got %v", resultNames(warnings))
	}
}

func TestWriteJSONReport(t *testing.T) {
	var buf bytes.Buffer
	rep := report.Report{
		Mode:    "check",
		OS:      "darwin",
		LogPath: "/tmp/test.log",
	}
	writeJSONReport(&buf, rep, redactor.New())
	out := buf.String()
	if !strings.Contains(out, "JSON report") {
		t.Error("expected 'JSON report' header")
	}
	if !strings.Contains(out, `"mode"`) {
		t.Error("expected JSON content with mode field")
	}
}

func TestPrintHumanShowsRisksWithoutResults(t *testing.T) {
	var buf bytes.Buffer
	rep := report.Report{
		Mode:    "check",
		LogPath: "/tmp/test.log",
		Risks: []report.Risk{
			{Provider: "mcp", Name: "spotify", Level: "manual", Reason: "manual review"},
		},
	}
	printHuman(&buf, rep, redactor.New(), true)
	out := buf.String()
	// manual-level risks appear under "Advisory" section
	if !strings.Contains(out, "Advisory") {
		t.Fatalf("expected Advisory section, got: %s", out)
	}
	if !strings.Contains(out, "spotify") {
		t.Fatalf("expected risk name in output, got: %s", out)
	}
}

func TestPrintHumanGroupsUpdateOutput(t *testing.T) {
	var buf bytes.Buffer
	rep := report.Report{
		Mode:      "update",
		LogPath:   "/tmp/update.log",
		BackupDir: "/tmp/backup",
		Summary:   report.Summary{Success: 3, Warning: 2},
		Results: []report.TaskResult{
			{Name: "backup-configs", Provider: "backup", Status: report.StatusSuccess, Summary: "backed up 3 configs"},
			{Name: "codex-update", Provider: "codex", Status: report.StatusSuccess, Summary: "updated codex"},
			{Name: "codex-version", Provider: "codex", Status: report.StatusSuccess, Summary: "codex 1.2.3"},
			{Name: "claude-mcp-list-after", Provider: "claude", Status: report.StatusWarning, Summary: "Checking MCP server health..."},
			{Name: "mcp-config-scan", Provider: "mcp", Status: report.StatusWarning, Summary: "partial scan warning"},
		},
		Risks: []report.Risk{
			{Provider: "mcp", Name: "spotify", Level: "manual", Reason: "manual review"},
		},
	}
	printHuman(&buf, rep, redactor.New(), true)
	out := buf.String()
	for _, want := range []string{"Actions", "Warnings", "Checks", "Details", "Advisory", "codex-update", "backup-configs", "manual review"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
	// claude-mcp-list-after is classified as a check; must not duplicate in Warnings
	if strings.Count(out, "claude-mcp-list-after") > 1 {
		t.Fatal("claude-mcp-list-after should appear only once, not in both Checks and Warnings")
	}
}


func TestPrintHumanNoAnsiInNonTTY(t *testing.T) {
	// When writing to a non-TTY (buf), no ANSI escapes should leak.
	var buf bytes.Buffer
	rep := report.Report{
		Mode:    "check",
		LogPath: "/tmp/test.log",
		Risks: []report.Risk{
			{Provider: "test", Name: "alpha", Level: "high", Reason: "critical", Path: "/tmp/alpha"},
			{Provider: "test", Name: "beta", Level: "medium", Reason: "moderate", Path: "/tmp/beta"},
			{Provider: "test", Name: "gamma", Level: "manual", Reason: "info", Path: "/tmp/gamma"},
		},
		Results: []report.TaskResult{
			{Name: "check-version", Provider: "test", Status: report.StatusSuccess, Summary: "v1.0"},
			{Name: "check-mcp", Provider: "test", Status: report.StatusWarning, Summary: "unreachable"},
		},
		Summary: report.Summary{Success: 1, Warning: 1},
	}
	printHuman(&buf, rep, redactor.New(), true)
	out := buf.String()
	assertNoAnsi(t, out)
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") || !strings.Contains(out, "gamma") {
		t.Errorf("expected risk names in output: %s", out)
	}
}

func assertNoAnsi(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "\033[") {
		t.Errorf("ANSI escape codes found in non-TTY output")
	}
}

// stubProvider implements provider.Provider for testing.
type stubProvider struct {
	name string
}

func (s stubProvider) Name() string { return s.name }
func (s stubProvider) Inventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	return nil, nil, nil
}
func (s stubProvider) UpdateTasks() []runner.Task            { return nil }
func (s stubProvider) PostUpdateChecks() []report.TaskResult { return nil }

func providerNames(providers []provider.Provider) []string {
	out := make([]string, len(providers))
	for i, p := range providers {
		out[i] = p.Name()
	}
	return out
}

func resultNames(results []report.TaskResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Name
	}
	return out
}

func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), err
}

func TestRunVersion(t *testing.T) {
	out, err := captureStdout(func() error { return Run([]string{"--version"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "update-ai-tools") {
		t.Errorf("expected version in output, got %q", out)
	}
}

func TestRunCheckJson(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	claudeHome := filepath.Join(home, ".claude")
	os.MkdirAll(filepath.Join(codexHome, "skills"), 0o700)
	os.MkdirAll(filepath.Join(claudeHome, "skills"), 0o700)

	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--check", "--json", "--only", "skills"})
	})
	if err != nil {
		t.Fatal(err)
	}

	var rep report.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", err, out)
	}
	if rep.Mode != "check" {
		t.Errorf("expected mode check, got %q", rep.Mode)
	}
	if rep.Home != home {
		t.Errorf("expected home %q in report, got %q", home, rep.Home)
	}
	if len(rep.Inventory) == 0 {
		t.Error("expected non-empty inventory")
	}
}

func TestRunJsonWithoutActionDefaultsToCheck(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".codex", "skills"), 0o700)
	os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o700)

	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--json", "--only", "skills"})
	})
	if err != nil {
		t.Fatal(err)
	}

	var rep report.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", err, out)
	}
	if rep.Mode != "check" {
		t.Fatalf("expected --json without action to run check mode, got %q", rep.Mode)
	}
	for _, result := range rep.Results {
		if result.Name == "backup-configs" || strings.Contains(result.Name, "update") {
			t.Fatalf("non-action --json should not back up or update, got result %+v", result)
		}
	}
}

func TestRunRejectsUnknownProviderFilter(t *testing.T) {
	home := t.TempDir()
	err := Run([]string{"--home", home, "--check", "--only", "ghost"})
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
	if !strings.Contains(err.Error(), "unknown provider") || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected unknown provider error naming ghost, got %v", err)
	}
}

func TestRunUpdateBlocksWhenBackupFails(t *testing.T) {
	home := t.TempDir()
	blockerParent := filepath.Join(home, ".codex", "backups")
	if err := os.MkdirAll(blockerParent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockerParent, "update-ai-tools"), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--update", "--json", "--only", "skills"})
	})
	if err == nil {
		t.Fatal("expected update to return an error when backup fails")
	}
	if !strings.Contains(err.Error(), "backup did not complete") {
		t.Fatalf("expected backup failure error, got %v", err)
	}

	var rep report.Report
	if unmarshalErr := json.Unmarshal([]byte(out), &rep); unmarshalErr != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", unmarshalErr, out)
	}
	if rep.Mode != "update" {
		t.Fatalf("expected update mode, got %q", rep.Mode)
	}
	var sawBackupFailure, sawUpdateSkip bool
	for _, result := range rep.Results {
		if result.Name == "backup-configs" && result.Status == report.StatusFailed {
			sawBackupFailure = true
		}
		if result.Name == "updates-skipped" && result.Status == report.StatusSkipped {
			sawUpdateSkip = true
		}
		if result.Name == "skills-update-global" {
			t.Fatalf("update task should not run after backup failure: %+v", result)
		}
	}
	if !sawBackupFailure || !sawUpdateSkip {
		t.Fatalf("expected backup failure and updates-skipped results, got %+v", rep.Results)
	}
}

func TestRunRejectsUnknownOnlyProvider(t *testing.T) {
	err := Run([]string{"--home", t.TempDir(), "--check", "--json", "--only", "definitely-not-a-provider"})
	if err == nil {
		t.Fatal("expected unknown --only provider to return an error")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("expected unknown provider error, got: %v", err)
	}
}

func TestRunRejectsUnknownSkipProvider(t *testing.T) {
	err := Run([]string{"--home", t.TempDir(), "--check", "--json", "--skip", "definitely-not-a-provider"})
	if err == nil {
		t.Fatal("expected unknown --skip provider to return an error")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("expected unknown provider error, got: %v", err)
	}
}

func TestRunDoesNotUpdateWhenBackupFails(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.Mkdir(configPath, 0o700); err != nil {
		t.Fatal(err)
	}

	_, marker := installFakeNpx(t, 0)
	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--update", "--json", "--only", "skills"})
	})
	if err == nil {
		t.Fatal("expected backup warning to return an error")
	}
	if !strings.Contains(err.Error(), "backup did not complete") {
		t.Fatalf("expected backup gating error, got %v", err)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("expected backup warning to block update command; marker stat error=%v", err)
	}
	var rep report.Report
	if unmarshalErr := json.Unmarshal([]byte(out), &rep); unmarshalErr != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", unmarshalErr, out)
	}
	assertTaskStatus(t, rep.Results, "backup-configs", report.StatusFailed)
	assertTaskStatus(t, rep.Results, "updates-skipped", report.StatusSkipped)
}

func TestRunDoesNotUpdateWhenPartialBackupWarnsWithoutForce(t *testing.T) {
	home := createPartialBackupWarningHome(t)
	_, marker := installFakeNpx(t, 0)

	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--update", "--json", "--only", "skills"})
	})
	if err == nil {
		t.Fatal("expected partial backup warning to return an error without --force")
	}
	if !strings.Contains(err.Error(), "backup did not complete") {
		t.Fatalf("expected backup gating error, got %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("expected backup warning to block update command; marker stat error=%v", err)
	}
	var rep report.Report
	if unmarshalErr := json.Unmarshal([]byte(out), &rep); unmarshalErr != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", unmarshalErr, out)
	}
	assertTaskStatus(t, rep.Results, "backup-configs", report.StatusWarning)
	assertTaskStatus(t, rep.Results, "updates-skipped", report.StatusSkipped)
}

func TestRunUpdateReturnsErrorWhenTaskFails(t *testing.T) {
	home := t.TempDir()
	installFakeNpx(t, 7)

	_, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--update", "--json", "--only", "skills"})
	})
	if err == nil {
		t.Fatal("expected non-interactive update to return an error when an update task fails")
	}
	if !strings.Contains(err.Error(), "failed task") {
		t.Fatalf("expected failed task error, got: %v", err)
	}
}

func TestRunUpdateReturnsErrorWhenSelectedUpdateTaskIsSkipped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir())

	_, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--update", "--json", "--only", "skills"})
	})
	if err == nil {
		t.Fatal("expected skipped selected update task to return an error")
	}
	if !strings.Contains(err.Error(), "skipped update task") || !strings.Contains(err.Error(), "skills-update-global") {
		t.Fatalf("expected skipped update task name in error, got: %v", err)
	}
}

func TestRunUpdateForceContinuesAfterPartialBackupWarning(t *testing.T) {
	home := createPartialBackupWarningHome(t)
	_, marker := installFakeNpx(t, 0)
	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--update", "--force", "--json", "--only", "skills"})
	})
	if err != nil {
		t.Fatalf("expected --force to continue after partial backup warning, got %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected update command to run with --force: %v", err)
	}
	var rep report.Report
	if unmarshalErr := json.Unmarshal([]byte(out), &rep); unmarshalErr != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", unmarshalErr, out)
	}
	assertTaskStatus(t, rep.Results, "backup-configs", report.StatusWarning)
	assertTaskStatus(t, rep.Results, "skills-update-global", report.StatusSuccess)
}

func TestRunCheckAndDryRunExclusive(t *testing.T) {
	err := Run([]string{"--check", "--dry-run"})
	if err == nil {
		t.Fatal("expected error for --check --dry-run together")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got: %v", err)
	}
}

func TestRunMenuRequiresTerminal(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()
	_ = w.Close()

	err = Run([]string{"--menu"})
	if err == nil {
		t.Fatal("expected --menu to require a terminal in tests")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Errorf("expected terminal error, got: %v", err)
	}
}

func TestInteractiveSelectReturnsErrorOnEOF(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()
	_ = w.Close()

	_, err = interactiveSelect()
	if err == nil {
		t.Fatal("expected EOF error")
	}
	if !strings.Contains(err.Error(), "read menu selection") {
		t.Errorf("expected read menu selection error, got: %v", err)
	}
}

func installFakeNpx(t *testing.T, exitCode int) (string, string) {
	t.Helper()
	binDir := t.TempDir()
	marker := filepath.Join(binDir, "npx-invoked")
	script := filepath.Join(binDir, "npx")
	body := "#!/bin/sh\nprintf invoked > " + shellQuote(marker) + "\nexit " + string(rune('0'+exitCode)) + "\n"
	if exitCode > 9 {
		t.Fatalf("test helper only supports one-digit exit codes, got %d", exitCode)
	}
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	return binDir, marker
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func createPartialBackupWarningHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("model = 'test'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(home, ".claude", "settings.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	return home
}

func assertTaskStatus(t *testing.T, results []report.TaskResult, name string, status report.Status) {
	t.Helper()
	for _, result := range results {
		if result.Name == name {
			if result.Status != status {
				t.Fatalf("expected %s status %s, got %s", name, status, result.Status)
			}
			return
		}
	}
	t.Fatalf("expected task %s in results, got %+v", name, results)
}

func TestDefaultArgsFallsBackToCheckWhenNoTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func() bool { return false }
	defer func() { isTerminal = orig }()

	args := defaultArgs([]string{})
	if len(args) != 1 || args[0] != "--check" {
		t.Errorf("expected [--check], got %v", args)
	}
}

func TestDefaultArgsUsesMenuWhenTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = orig }()

	args := defaultArgs([]string{})
	if len(args) != 1 || args[0] != "--menu" {
		t.Errorf("expected [--menu], got %v", args)
	}
}

func TestDefaultArgsPreservesExplicitArgs(t *testing.T) {
	// should not depend on terminal state when args are explicit
	orig := isTerminal
	isTerminal = func() bool { return false }
	defer func() { isTerminal = orig }()

	args := defaultArgs([]string{"--check", "--json"})
	if len(args) != 2 || args[0] != "--check" || args[1] != "--json" {
		t.Errorf("expected [--check --json], got %v", args)
	}
}

func TestRunExplicitMenuErrorsWithoutTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func() bool { return false }
	defer func() { isTerminal = orig }()

	err := Run([]string{"--menu"})
	if err == nil {
		t.Fatal("expected --menu to fail without a terminal")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Errorf("expected interactive terminal error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--check") {
		t.Errorf("expected suggestion to use --check, got: %v", err)
	}
}

func TestPrintHumanNonVerboseHidesChecksDetailsAdvisory(t *testing.T) {
	var buf bytes.Buffer
	rep := report.Report{
		Mode:      "update",
		LogPath:   "/tmp/update.log",
		BackupDir: "/tmp/backup",
		Summary:   report.Summary{Success: 4, Warning: 2, Failed: 1},
		Results: []report.TaskResult{
			{Name: "backup-configs", Provider: "backup", Status: report.StatusSuccess, Summary: "backed up 3 configs"},
			{Name: "codex-update", Provider: "codex", Status: report.StatusSuccess, Summary: "updated codex"},
			{Name: "codex-version", Provider: "codex", Status: report.StatusSuccess, Summary: "codex 1.2.3"},
			{Name: "claude-doctor", Provider: "claude", Status: report.StatusSuccess, Summary: "claude healthy"},
			{Name: "claude-mcp-list-after", Provider: "claude", Status: report.StatusWarning, Summary: "Checking MCP server health..."},
			{Name: "mcp-config-scan", Provider: "mcp", Status: report.StatusWarning, Summary: "partial scan warning"},
		},
		Risks: []report.Risk{
			{Provider: "mcp", Name: "spotify", Level: "high", Reason: "critical issue", Path: "/tmp/spotify"},
			{Provider: "mcp", Name: "xhs", Level: "manual", Reason: "manual review", Path: "/tmp/xhs"},
		},
	}
	printHuman(&buf, rep, redactor.New(), false)
	out := buf.String()
	assertNoAnsi(t, out)

	// must show: summary bar, Actions, Warnings, actionable Risks, log/backup paths
	for _, want := range []string{"Actions", "codex-update", "Warnings", "/tmp/backup"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in non-verbose output:\n%s", want, out)
		}
	}
	// actionable risks should appear
	if !strings.Contains(out, "Risks") {
		t.Fatal("expected Risks section in non-verbose output")
	}
	if !strings.Contains(out, "critical issue") {
		t.Fatal("expected actionable risk in non-verbose output")
	}

	// must hide: Checks, Details, Advisory
	for _, unwanted := range []string{"Checks", "Details", "Advisory", "codex-version", "claude-doctor"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("unexpected %q in non-verbose output:\n%s", unwanted, out)
		}
	}
	// manual risk should not appear (only in Advisory)
	if strings.Contains(out, "manual review") {
		t.Fatal("advisory risk should be hidden in non-verbose mode")
	}
}

func TestPrintHumanVerboseShowsAllSections(t *testing.T) {
	var buf bytes.Buffer
	rep := report.Report{
		Mode:    "check",
		LogPath: "/tmp/test.log",
		Summary: report.Summary{Success: 2},
		Results: []report.TaskResult{
			{Name: "backup-configs", Provider: "backup", Status: report.StatusSuccess, Summary: "backed up"},
			{Name: "codex-version", Provider: "codex", Status: report.StatusSuccess, Summary: "codex 1.0"},
			{Name: "claude-mcp-list", Provider: "claude", Status: report.StatusSuccess, Summary: "2 servers"},
		},
		Risks: []report.Risk{
			{Provider: "mcp", Name: "spotify", Level: "manual", Reason: "manual review"},
		},
	}
	printHuman(&buf, rep, redactor.New(), true)
	out := buf.String()
	assertNoAnsi(t, out)

	for _, want := range []string{"Checks", "Details", "Advisory", "codex-version", "backed up", "manual review"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in verbose output:\n%s", want, out)
		}
	}
}

func TestPrintHumanNonVerboseNoSectionsWithoutContent(t *testing.T) {
	// when there are no warnings or risks, those sections should not render at all
	var buf bytes.Buffer
	rep := report.Report{
		Mode:    "check",
		LogPath: "/tmp/test.log",
		Summary: report.Summary{Success: 0},
	}
	printHuman(&buf, rep, redactor.New(), false)
	out := buf.String()
	for _, unwanted := range []string{"Actions", "Warnings", "Risks"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("empty section %q should not render:\n%s", unwanted, out)
		}
	}
}

func TestRunVerboseShowsInfoToConsole(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".codex", "skills"), 0o700)

	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--check", "--verbose", "--only", "skills"})
	})
	if err != nil {
		t.Fatal(err)
	}
	// verbose should show INFO log lines on console
	if !strings.Contains(out, "INFO") {
		t.Error("expected INFO log lines in verbose output")
	}
}

func TestRunNonVerboseHidesInfoFromConsole(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".codex", "skills"), 0o700)

	out, err := captureStdout(func() error {
		return Run([]string{"--home", home, "--check", "--only", "skills"})
	})
	if err != nil {
		t.Fatal(err)
	}
	// non-verbose should not show INFO log lines on console
	if strings.Contains(out, "INFO") {
		t.Error("expected no INFO log lines in non-verbose console output")
	}
}
