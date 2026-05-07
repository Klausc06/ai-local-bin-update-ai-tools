package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanConfigRisksDetectsSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(`{"api_key":"sk-abc123","token":"xyz","Authorization":"Bearer deadbeef"}`), 0o600)

	risks := scanConfigRisks("test", path)
	if len(risks) == 0 {
		t.Fatal("expected at least one risk for sensitive content")
	}
	found := false
	for _, r := range risks {
		if r.Level == "sensitive" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a sensitive-level risk")
	}
}

func TestScanConfigRisksDetectsXiaohongshu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(`{"mcpServers":{"xiaohongshu":{"command":"node"}}}`), 0o600)

	risks := scanConfigRisks("test", path)
	found := false
	for _, r := range risks {
		if strings.Contains(strings.ToLower(r.Name), "xiaohongshu") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected xiaohongshu risk flag")
	}
}

func TestScanConfigRisksDetectsSpotify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(`{"mcpServers":{"spotify":{"command":"python"}}}`), 0o600)

	risks := scanConfigRisks("test", path)
	found := false
	for _, r := range risks {
		if strings.Contains(strings.ToLower(r.Name), "spotify") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected spotify risk flag")
	}
}

func TestScanConfigRisksCleanFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.json")
	os.WriteFile(path, []byte(`{"name":"test","version":"1.0"}`), 0o600)

	risks := scanConfigRisks("test", path)
	if len(risks) != 0 {
		t.Errorf("expected no risks for clean file, got %d", len(risks))
	}
}

func TestScanConfigRisksMissingFile(t *testing.T) {
	risks := scanConfigRisks("test", "/nonexistent/path/config.json")
	if len(risks) != 0 {
		t.Errorf("expected no risks for missing file, got %d", len(risks))
	}
}

func TestRisksFromMCPOutputXiaohongshu(t *testing.T) {
	risks := risksFromMCPOutput("codex", `{"mcp_servers":[{"name":"xiaohongshu","status":"connected"}]}`)
	found := false
	for _, r := range risks {
		if r.Name == "xiaohongshu" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected xiaohongshu risk from MCP output")
	}
}

func TestRisksFromMCPOutputSpotify(t *testing.T) {
	risks := risksFromMCPOutput("codex", `{"mcp_servers":[{"name":"spotify","status":"connected"}]}`)
	found := false
	for _, r := range risks {
		if r.Name == "spotify" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected spotify risk from MCP output")
	}
}

func TestRisksFromMCPOutputSensitive(t *testing.T) {
	risks := risksFromMCPOutput("codex", `some output with api_key and token references`)
	found := false
	for _, r := range risks {
		if r.Level == "sensitive" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sensitive risk from MCP output")
	}
}

func TestRisksFromMCPOutputClean(t *testing.T) {
	risks := risksFromMCPOutput("codex", `all systems operational`)
	if len(risks) != 0 {
		t.Errorf("expected no risks for clean output, got %d", len(risks))
	}
}

func TestClassifyMCPInFileURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(`{"url":"https://mcp.example.com/sse"}`), 0o600)

	items := classifyMCPInFile("test", path)
	found := false
	for _, item := range items {
		if item.Type == "mcp-url" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mcp-url item")
	}
}

func TestClassifyMCPInFileCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(`{"command":"npx","args":["-y","@anthropic/mcp-server"]}`), 0o600)

	items := classifyMCPInFile("test", path)
	found := false
	for _, item := range items {
		if item.Type == "mcp-command" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mcp-command item")
	}
}

func TestClassifyMCPInFileLocalService(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(`{"command":"/usr/local/bin/mcp-server","args":["--host","localhost","--port","3000"]}`), 0o600)

	items := classifyMCPInFile("test", path)
	found := false
	for _, item := range items {
		if item.Type == "mcp-local-service" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mcp-local-service item")
	}
}

func TestClassifyMCPInFileMissing(t *testing.T) {
	items := classifyMCPInFile("test", "/nonexistent/mcp.json")
	if len(items) != 0 {
		t.Errorf("expected no items for missing file, got %d", len(items))
	}
}

func TestCountFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o600)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o700)
	os.WriteFile(filepath.Join(dir, "sub", "c.txt"), []byte("c"), 0o600)

	count := countFiles(dir, func(string) bool { return true })
	if count != 3 {
		t.Errorf("expected 3 files, got %d", count)
	}
}

func TestCountFilesWithFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0o600)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o600)

	count := countFiles(dir, func(p string) bool { return filepath.Ext(p) == ".go" })
	if count != 1 {
		t.Errorf("expected 1 .go file, got %d", count)
	}
}

func TestCountFilesMissing(t *testing.T) {
	count := countFiles("/nonexistent/path", func(string) bool { return true })
	if count != 0 {
		t.Errorf("expected 0 files for missing dir, got %d", count)
	}
}

func TestJSONSize(t *testing.T) {
	if n := jsonSize([]any{1, 2, 3}); n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
	if n := jsonSize(map[string]any{"a": 1, "b": 2}); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
	if n := jsonSize("string"); n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	if exists(filepath.Join(dir, "missing")) {
		t.Error("expected false for missing file")
	}
	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("ok"), 0o600)
	if !exists(filepath.Join(dir, "exists.txt")) {
		t.Error("expected true for existing file")
	}
}
