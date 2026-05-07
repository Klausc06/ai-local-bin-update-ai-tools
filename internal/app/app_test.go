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
		check, dryRun bool
		want          string
	}{
		{false, false, "update"},
		{true, false, "check"},
		{false, true, "dry-run"},
	}
	for _, c := range cases {
		got := modeName(c.check, c.dryRun)
		if got != c.want {
			t.Errorf("modeName(%v,%v) = %q, want %q", c.check, c.dryRun, got, c.want)
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
	if opts.check || opts.dryRun || opts.version || opts.jsonOut || opts.verbose {
		t.Fatal("expected all flags to be false by default")
	}
	if len(opts.only) != 0 || len(opts.skip) != 0 {
		t.Fatal("expected empty only/skip")
	}
}

func TestDefaultArgsUsesMenu(t *testing.T) {
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
	printHuman(&buf, rep, redactor.New())
	out := buf.String()
	if !strings.Contains(out, "1 risk(s)") {
		t.Fatalf("expected risk section, got: %s", out)
	}
	if !strings.Contains(out, "spotify") {
		t.Fatalf("expected risk name in output, got: %s", out)
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
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), nil
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
