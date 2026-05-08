package backup

import (
	"os"
	"path/filepath"
	"testing"

	"update-ai-tools/internal/platform"
	"update-ai-tools/internal/redactor"
	"update-ai-tools/internal/report"
)

func TestSafeName(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"/home/user/.codex/config.toml", "__home__user__.codex__config.toml"},
		{"/etc/passwd", "__etc__passwd"},
		{"simple", "simple"},
		{"/a/b/c", "__a__b__c"},
	}
	for _, c := range cases {
		got := safeName(c.input)
		if got != c.want {
			t.Errorf("safeName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSafeNameWithBackslashes(t *testing.T) {
	got := safeName(`C:\Users\test\.codex\config.toml`)
	if got == `C:\Users\test\.codex\config.toml` {
		t.Error("expected backslashes to be replaced")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst.txt")
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestCopyFileOverwriteFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	os.WriteFile(src, []byte("src"), 0o600)
	dst := filepath.Join(dir, "dst.txt")
	os.WriteFile(dst, []byte("dst"), 0o600)
	err := copyFile(src, dst)
	if err == nil {
		t.Fatal("expected error on overwrite (O_EXCL)")
	}
}

func TestConfigsCreatesBackup(t *testing.T) {
	home := t.TempDir()
	codex := filepath.Join(home, ".codex")
	os.MkdirAll(codex, 0o700)
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte("[test]"), 0o600)

	profile := platform.Detect(home)
	red := redactor.New()
	var buf testWriter
	log := report.NewLogger(&buf, &buf, red, true)

	dest, result := Configs(profile, red, log)
	if result.Status != report.StatusSuccess {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Summary)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("backup directory not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, safeName(filepath.Join(codex, "config.toml")))); err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
}

func TestConfigsSkipsMissingFiles(t *testing.T) {
	home := t.TempDir()
	// Don't create any config files
	profile := platform.Detect(home)
	red := redactor.New()
	var buf testWriter
	log := report.NewLogger(&buf, &buf, red, true)

	_, result := Configs(profile, red, log)
	if result.Status != report.StatusSuccess {
		t.Fatalf("expected success even with no files: %s", result.Summary)
	}
}

func TestConfigsFailsWhenNoExistingConfigCanBeCopied(t *testing.T) {
	home := t.TempDir()
	codex := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codex, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(codex, "config.toml"), 0o700); err != nil {
		t.Fatal(err)
	}

	profile := platform.Detect(home)
	red := redactor.New()
	var buf testWriter
	log := report.NewLogger(&buf, &buf, red, true)

	_, result := Configs(profile, red, log)
	if result.Status != report.StatusFailed {
		t.Fatalf("expected failed when every existing config copy fails, got %s: %s", result.Status, result.Summary)
	}
}

func TestConfigsWarnsWhenSomeExistingConfigsFail(t *testing.T) {
	home := t.TempDir()
	codex := filepath.Join(home, ".codex")
	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(codex, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claude, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codex, "config.toml"), []byte("[test]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(claude, "settings.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	profile := platform.Detect(home)
	red := redactor.New()
	var buf testWriter
	log := report.NewLogger(&buf, &buf, red, true)

	_, result := Configs(profile, red, log)
	if result.Status != report.StatusWarning {
		t.Fatalf("expected warning when only some config copies fail, got %s: %s", result.Status, result.Summary)
	}
}

type testWriter struct{}

func (testWriter) Write(p []byte) (int, error) { return len(p), nil }
