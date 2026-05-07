package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"update-ai-tools/internal/platform"
	"update-ai-tools/internal/report"
	"update-ai-tools/internal/runner"
)

type Provider interface {
	Name() string
	Inventory() ([]report.Item, []report.Risk, []report.TaskResult)
	UpdateTasks() []runner.Task
	PostUpdateChecks() []report.TaskResult
}

type baseProvider struct {
	name    string
	profile platform.Profile
	runner  *runner.Runner
}

func DefaultRegistry(profile platform.Profile, r *runner.Runner) []Provider {
	return []Provider{
		baseProvider{name: "codex", profile: profile, runner: r},
		baseProvider{name: "claude", profile: profile, runner: r},
		baseProvider{name: "omx", profile: profile, runner: r},
		baseProvider{name: "skills", profile: profile, runner: r},
		baseProvider{name: "workbuddy", profile: profile, runner: r},
		baseProvider{name: "mcp", profile: profile, runner: r},
	}
}

func (p baseProvider) Name() string { return p.name }

func (p baseProvider) Inventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	switch p.name {
	case "codex":
		return p.codexInventory()
	case "claude":
		return p.claudeInventory()
	case "omx":
		return p.omxInventory()
	case "skills":
		return p.skillsInventory()
	case "workbuddy":
		return p.workbuddyInventory()
	case "mcp":
		return p.mcpInventory()
	default:
		return nil, nil, nil
	}
}

func (p baseProvider) UpdateTasks() []runner.Task {
	switch p.name {
	case "codex":
		return []runner.Task{{Name: "codex-update", Provider: p.name, Command: []string{"codex", "update"}, SkipIfMissing: "codex", Timeout: 10 * time.Minute}}
	case "claude":
		return []runner.Task{{Name: "claude-update", Provider: p.name, Command: []string{"claude", "update"}, FallbackCommands: [][]string{{"claude", "upgrade"}}, SkipIfMissing: "claude", Timeout: 10 * time.Minute}}
	case "omx":
		return []runner.Task{{Name: "omx-update", Provider: p.name, Command: []string{"omx", "update"}, SkipIfMissing: "omx", Timeout: 10 * time.Minute}}
	case "skills":
		return []runner.Task{{Name: "skills-update-global", Provider: p.name, Command: []string{"npx", "skills", "update", "-g", "-y"}, SkipIfMissing: "npx", Timeout: 10 * time.Minute}}
	default:
		return nil
	}
}

func (p baseProvider) PostUpdateChecks() []report.TaskResult {
	switch p.name {
	case "codex":
		return []report.TaskResult{p.runner.Capture(p.name, "codex-mcp-list-after", 90*time.Second, "codex", "mcp", "list")}
	case "claude":
		return []report.TaskResult{p.runner.Capture(p.name, "claude-mcp-list-after", 90*time.Second, "claude", "mcp", "list")}
	case "omx":
		return []report.TaskResult{p.runner.Capture(p.name, "omx-doctor-after", 90*time.Second, "omx", "doctor")}
	default:
		return nil
	}
}

func (p baseProvider) codexInventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	items := []report.Item{dirItem(p.name, "home", p.profile.CodexHome)}
	items = append(items, countFilesItem(p.name, "agents", p.profile.CodexHome, "agents"))
	items = append(items, countSkillItem(p.name, filepath.Join(p.profile.CodexHome, "skills")))
	results := []report.TaskResult{
		p.runner.Capture(p.name, "codex-version", 30*time.Second, "codex", "--version"),
		p.runner.Capture(p.name, "codex-mcp-list", 90*time.Second, "codex", "mcp", "list"),
	}
	return items, risksFromMCPOutput(p.name, results[1].Output), results
}

func (p baseProvider) claudeInventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	items := []report.Item{dirItem(p.name, "home", p.profile.ClaudeHome)}
	items = append(items, countFilesItem(p.name, "agents", p.profile.ClaudeHome, "agents"))
	items = append(items, countSkillItem(p.name, filepath.Join(p.profile.ClaudeHome, "skills")))
	items = append(items, jsonCountItem(p.name, "plugins", filepath.Join(p.profile.ClaudeHome, "plugins", "installed_plugins.json")))
	results := []report.TaskResult{
		p.runner.Capture(p.name, "claude-version", 30*time.Second, "claude", "--version"),
		p.runner.Capture(p.name, "claude-mcp-list", 90*time.Second, "claude", "mcp", "list"),
	}
	return items, risksFromMCPOutput(p.name, results[1].Output), results
}

func (p baseProvider) omxInventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	items := []report.Item{
		countFilesItem(p.name, "agents", p.profile.CodexHome, "agents"),
		countSkillItem(p.name, filepath.Join(p.profile.CodexHome, "skills")),
	}
	results := []report.TaskResult{
		p.runner.Capture(p.name, "omx-version", 30*time.Second, "omx", "--version"),
		p.runner.Capture(p.name, "omx-doctor", 90*time.Second, "omx", "doctor"),
	}
	return items, nil, results
}

func (p baseProvider) skillsInventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	items := []report.Item{
		countSkillItem(p.name, filepath.Join(p.profile.CodexHome, "skills")),
		countSkillItem(p.name, filepath.Join(p.profile.ClaudeHome, "skills")),
		countSkillItem(p.name, filepath.Join(p.profile.AgentsHome, "skills")),
		countSkillItem(p.name, filepath.Join(p.profile.WorkBuddyHome, "skills")),
	}
	return items, nil, nil
}

func (p baseProvider) workbuddyInventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	items := []report.Item{dirItem(p.name, "home", p.profile.WorkBuddyHome)}
	for _, sub := range []string{"skills", "skills-marketplace", "connectors-marketplace", "plugins", "connectors"} {
		items = append(items, countFilesItem(p.name, sub, p.profile.WorkBuddyHome, sub))
	}
	risks := []report.Risk{}
	for _, sub := range []string{"skills-marketplace", "connectors-marketplace", "plugin-marketplace-state"} {
		path := filepath.Join(p.profile.WorkBuddyHome, sub)
		if exists(path) {
			risks = append(risks, report.Risk{Provider: p.name, Name: sub, Level: "manual", Path: path, Reason: "marketplace/cache content is reported but not auto-updated"})
		}
	}
	return items, risks, nil
}

func (p baseProvider) mcpInventory() ([]report.Item, []report.Risk, []report.TaskResult) {
	var items []report.Item
	var risks []report.Risk
	for _, path := range p.profile.ConfigFiles {
		if !exists(path) {
			continue
		}
		items = append(items, report.Item{Provider: p.name, Name: filepath.Base(path), Type: "config", Status: "present", Path: path})
		risks = append(risks, scanConfigRisks(p.name, path)...)
		items = append(items, classifyMCPInFile(p.name, path)...)
	}
	for _, dir := range p.profile.LaunchDirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*[Mm][Cc][Pp]*"))
		for _, path := range matches {
			items = append(items, report.Item{Provider: p.name, Name: filepath.Base(path), Type: "launch-service", Status: "present", Path: path})
			risks = append(risks, report.Risk{Provider: p.name, Name: filepath.Base(path), Level: "manual", Path: path, Reason: "LaunchAgent/service is reported but not modified"})
		}
	}
	return items, risks, nil
}

func dirItem(provider, name, path string) report.Item {
	status := "missing"
	if exists(path) {
		status = "present"
	}
	return report.Item{Provider: provider, Name: name, Type: "directory", Status: status, Path: path}
}

func countFilesItem(provider, name, root, sub string) report.Item {
	path := filepath.Join(root, sub)
	count := countFiles(path, func(string) bool { return true })
	status := "missing"
	if exists(path) {
		status = "present"
	}
	return report.Item{Provider: provider, Name: name, Type: "directory", Status: status, Path: path, Detail: fmt.Sprintf("%d files", count)}
}

func countSkillItem(provider, path string) report.Item {
	count := countFiles(path, func(path string) bool { return filepath.Base(path) == "SKILL.md" })
	status := "missing"
	if exists(path) {
		status = "present"
	}
	return report.Item{Provider: provider, Name: filepath.Base(filepath.Dir(path)) + "/skills", Type: "skills", Status: status, Path: path, Detail: fmt.Sprintf("%d skills", count)}
}

func jsonCountItem(provider, name, path string) report.Item {
	item := report.Item{Provider: provider, Name: name, Type: "json", Path: path, Status: "missing"}
	data, err := os.ReadFile(path)
	if err != nil {
		return item
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		item.Status = "invalid"
		item.Detail = err.Error()
		return item
	}
	item.Status = "present"
	item.Detail = fmt.Sprintf("%d entries", jsonSize(v))
	return item
}

func jsonSize(v any) int {
	switch x := v.(type) {
	case []any:
		return len(x)
	case map[string]any:
		return len(x)
	default:
		return 1
	}
}

func countFiles(path string, match func(string) bool) int {
	count := 0
	_ = filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if match(path) {
			count++
		}
		return nil
	})
	return count
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func scanConfigRisks(provider, path string) []report.Risk {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(data)
	var risks []report.Risk
	if regexp.MustCompile(`(?i)(token|secret|api[_-]?key|authorization|bearer|password)`).MatchString(text) {
		risks = append(risks, report.Risk{Provider: provider, Name: filepath.Base(path), Level: "sensitive", Path: path, Reason: "contains secret-like keys; report only, never auto-edit"})
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "xiaohongshu") || strings.Contains(lower, "小红书") {
		risks = append(risks, report.Risk{Provider: provider, Name: "xiaohongshu", Level: "manual", Path: path, Reason: "local Xiaohongshu MCP is high risk and not auto-updated"})
	}
	if strings.Contains(lower, "spotify") {
		risks = append(risks, report.Risk{Provider: provider, Name: "spotify", Level: "manual", Path: path, Reason: "hand-written Spotify MCP is high risk and not auto-updated"})
	}
	return risks
}

func classifyMCPInFile(provider, path string) []report.Item {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(data)
	var items []report.Item
	if regexp.MustCompile(`https?://`).MatchString(text) {
		items = append(items, report.Item{Provider: provider, Name: filepath.Base(path), Type: "mcp-url", Status: "detected", Path: path})
	}
	if regexp.MustCompile(`(?i)(command|args|stdio|npx|node|python|bash|powershell|pwsh)`).MatchString(text) {
		items = append(items, report.Item{Provider: provider, Name: filepath.Base(path), Type: "mcp-command", Status: "detected", Path: path})
	}
	if regexp.MustCompile(`(?i)(localhost|127\.0\.0\.1|launchagent|systemd|service)`).MatchString(text) {
		items = append(items, report.Item{Provider: provider, Name: filepath.Base(path), Type: "mcp-local-service", Status: "detected", Path: path})
	}
	return items
}

func risksFromMCPOutput(provider, output string) []report.Risk {
	var risks []report.Risk
	lower := strings.ToLower(output)
	if strings.Contains(lower, "xiaohongshu") {
		risks = append(risks, report.Risk{Provider: provider, Name: "xiaohongshu", Level: "manual", Reason: "local Xiaohongshu MCP is high risk and not auto-updated"})
	}
	if strings.Contains(lower, "spotify") {
		risks = append(risks, report.Risk{Provider: provider, Name: "spotify", Level: "manual", Reason: "hand-written Spotify MCP is high risk and not auto-updated"})
	}
	if strings.Contains(lower, "token") || strings.Contains(lower, "api_key") || strings.Contains(lower, "secret") {
		risks = append(risks, report.Risk{Provider: provider, Name: "mcp-output", Level: "sensitive", Reason: "MCP list contains secret-like fields; output is redacted"})
	}
	return risks
}

func init() {
	_ = runtime.GOOS
	_ = exec.ErrNotFound
}
