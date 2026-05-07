package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"update-ai-tools/internal/platform"
	"update-ai-tools/internal/report"
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

func TestDirItem(t *testing.T) {
	item := dirItem("test", "home", "/some/path")
	if item.Provider != "test" || item.Name != "home" || item.Type != "directory" {
		t.Errorf("unexpected item: %+v", item)
	}
	if item.Status != "missing" {
		t.Errorf("expected missing status for nonexistent path, got %s", item.Status)
	}
	if item.Path != "/some/path" {
		t.Errorf("expected /some/path, got %q", item.Path)
	}
}

func TestCountFilesItem(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o600)

	item := countFilesItem("test", "agents", dir, ".")
	if item.Detail != "2 files" {
		t.Errorf("expected '2 files', got %q", item.Detail)
	}
	if item.Status != "present" {
		t.Errorf("expected present, got %s", item.Status)
	}
}

func TestCountSkillItem(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	os.MkdirAll(skillsDir, 0o700)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# skill"), 0o600)
	os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("readme"), 0o600)

	item := countSkillItem("test", skillsDir)
	if item.Type != "skills" {
		t.Errorf("expected type skills, got %s", item.Type)
	}
	if item.Detail != "1 skills" {
		t.Errorf("expected '1 skills', got %q", item.Detail)
	}
}

func TestSkillsInventory(t *testing.T) {
	home := t.TempDir()
	profile := platform.Profile{
		Home:          home,
		CodexHome:     filepath.Join(home, ".codex"),
		ClaudeHome:    filepath.Join(home, ".claude"),
		AgentsHome:    filepath.Join(home, ".agents"),
		WorkBuddyHome: filepath.Join(home, ".workbuddy"),
	}
	for _, dir := range []string{
		filepath.Join(profile.CodexHome, "skills"),
		filepath.Join(profile.ClaudeHome, "skills"),
		filepath.Join(profile.AgentsHome, "skills"),
		filepath.Join(profile.WorkBuddyHome, "skills"),
	} {
		os.MkdirAll(dir, 0o700)
	}

	p := baseProvider{name: "skills", profile: profile}
	items, risks, results := p.Inventory()

	if len(items) != 4 {
		t.Fatalf("expected 4 inventory items, got %d", len(items))
	}
	if risks != nil {
		t.Fatalf("expected no risks, got %d", len(risks))
	}
	if results != nil {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestWorkbuddyInventoryRisks(t *testing.T) {
	home := t.TempDir()
	profile := platform.Profile{
		Home:          home,
		WorkBuddyHome: filepath.Join(home, ".workbuddy"),
	}
	marketplace := filepath.Join(profile.WorkBuddyHome, "skills-marketplace")
	os.MkdirAll(marketplace, 0o700)

	p := baseProvider{name: "workbuddy", profile: profile}
	_, risks, _ := p.Inventory()

	found := false
	for _, r := range risks {
		if r.Name == "skills-marketplace" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected marketplace risk")
	}
}

// stubRunner implements TaskRunner for inventory tests.
type stubRunner struct {
	results map[string]report.TaskResult
}

func (r *stubRunner) Capture(provider, name string, timeout time.Duration, command ...string) report.TaskResult {
	if res, ok := r.results[name]; ok {
		return res
	}
	return report.TaskResult{Name: name, Provider: provider, Status: report.StatusSuccess}
}

func TestCodexInventory(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	os.MkdirAll(filepath.Join(codexHome, "agents"), 0o700)
	skillsDir := filepath.Join(codexHome, "skills")
	os.MkdirAll(skillsDir, 0o700)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("skill"), 0o600)

	stub := &stubRunner{
		results: map[string]report.TaskResult{
			"codex-version":  {Name: "codex-version", Status: report.StatusSuccess, Output: "0.128.0"},
			"codex-mcp-list": {Name: "codex-mcp-list", Status: report.StatusSuccess, Output: "no risks"},
		},
	}
	p := baseProvider{name: "codex", profile: platform.Profile{Home: home, CodexHome: codexHome}, runner: stub}

	items, risks, results := p.Inventory()

	if len(items) < 3 {
		t.Fatalf("expected at least 3 items, got %d", len(items))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(risks) > 0 {
		t.Errorf("expected no risks for clean mcp output, got %d", len(risks))
	}
	types := map[string]bool{}
	for _, item := range items {
		types[item.Type] = true
	}
	if !types["directory"] || !types["skills"] {
		t.Errorf("expected directory and skills item types, got %v", types)
	}
}

func TestCodexInventoryDetectsRisks(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	os.MkdirAll(codexHome, 0o700)

	stub := &stubRunner{
		results: map[string]report.TaskResult{
			"codex-version":  {Name: "codex-version", Status: report.StatusSuccess, Output: "0.128.0"},
			"codex-mcp-list": {Name: "codex-mcp-list", Status: report.StatusSuccess, Output: "xiaohongshu connected; token in config"},
		},
	}
	p := baseProvider{name: "codex", profile: platform.Profile{Home: home, CodexHome: codexHome}, runner: stub}

	_, risks, _ := p.Inventory()

	names := map[string]bool{}
	for _, r := range risks {
		names[r.Name] = true
	}
	if !names["xiaohongshu"] {
		t.Error("expected xiaohongshu risk")
	}
	if !names["mcp-output"] {
		t.Error("expected sensitive risk")
	}
}

func TestClaudeInventory(t *testing.T) {
	home := t.TempDir()
	claudeHome := filepath.Join(home, ".claude")
	os.MkdirAll(filepath.Join(claudeHome, "agents"), 0o700)
	skillsDir := filepath.Join(claudeHome, "skills")
	os.MkdirAll(skillsDir, 0o700)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("skill"), 0o600)
	pluginsDir := filepath.Join(claudeHome, "plugins")
	os.MkdirAll(pluginsDir, 0o700)
	os.WriteFile(filepath.Join(pluginsDir, "installed_plugins.json"), []byte(`{"plugins":["a","b","c"]}`), 0o600)

	stub := &stubRunner{
		results: map[string]report.TaskResult{
			"claude-version":  {Name: "claude-version", Status: report.StatusSuccess, Output: "2.0.0"},
			"claude-mcp-list": {Name: "claude-mcp-list", Status: report.StatusSuccess, Output: "clean"},
		},
	}
	p := baseProvider{name: "claude", profile: platform.Profile{Home: home, ClaudeHome: claudeHome}, runner: stub}

	items, _, results := p.Inventory()

	if len(items) < 4 {
		t.Fatalf("expected at least 4 items (home, agents, skills, plugins), got %d", len(items))
	}
	hasPlugins := false
	for _, item := range items {
		if item.Type == "json" && item.Detail == "1 entries" {
			hasPlugins = true
		}
	}
	if !hasPlugins {
		t.Error("expected plugins item with entry count")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestOmxInventory(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	os.MkdirAll(filepath.Join(codexHome, "agents"), 0o700)
	os.MkdirAll(filepath.Join(codexHome, "skills"), 0o700)

	stub := &stubRunner{
		results: map[string]report.TaskResult{
			"omx-version": {Name: "omx-version", Status: report.StatusSuccess, Output: "1.5.0"},
			"omx-doctor":  {Name: "omx-doctor", Status: report.StatusSuccess, Output: "all ok"},
		},
	}
	p := baseProvider{name: "omx", profile: platform.Profile{Home: home, CodexHome: codexHome}, runner: stub}

	items, _, results := p.Inventory()

	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(items))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestMcpInventory(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	os.MkdirAll(codexHome, 0o700)
	configFile := filepath.Join(codexHome, "config.toml")
	os.WriteFile(configFile, []byte("[mcp_servers]\nurl = \"https://mcp.example.com/sse\"\ncommand = \"npx\"\ntoken = \"sk-abc123\"\n"), 0o600)

	launchDir := filepath.Join(home, "Library", "LaunchAgents")
	os.MkdirAll(launchDir, 0o700)
	os.WriteFile(filepath.Join(launchDir, "com.user.mcp-server.plist"), []byte("plist"), 0o600)

	profile := platform.Profile{
		Home:        home,
		CodexHome:   codexHome,
		ConfigFiles: []string{configFile},
		LaunchDirs:  []string{launchDir},
	}
	p := baseProvider{name: "mcp", profile: profile}

	items, risks, results := p.Inventory()

	if results != nil {
		t.Fatalf("mcp inventory should have no task results, got %d", len(results))
	}

	hasConfig := false
	hasLaunch := false
	for _, item := range items {
		if item.Type == "config" {
			hasConfig = true
		}
		if item.Type == "launch-service" {
			hasLaunch = true
		}
	}
	if !hasConfig {
		t.Error("expected config item")
	}
	if !hasLaunch {
		t.Error("expected launch-service item")
	}

	hasSecret := false
	for _, risk := range risks {
		if risk.Level == "sensitive" {
			hasSecret = true
		}
	}
	if !hasSecret {
		t.Error("expected sensitive risk for config containing token/secret")
	}
}

func TestMcpInventorySkipsMissingConfigs(t *testing.T) {
	home := t.TempDir()
	profile := platform.Profile{
		Home:        home,
		CodexHome:   filepath.Join(home, ".codex"),
		ConfigFiles: []string{filepath.Join(home, ".codex", "nonexistent.json")},
		LaunchDirs:  []string{},
	}
	p := baseProvider{name: "mcp", profile: profile}

	items, risks, results := p.Inventory()

	if len(items) != 0 {
		t.Errorf("expected no items for missing configs, got %d", len(items))
	}
	if len(risks) != 0 {
		t.Errorf("expected no risks, got %d", len(risks))
	}
	if results != nil {
		t.Errorf("expected nil results, got %d", len(results))
	}
}
