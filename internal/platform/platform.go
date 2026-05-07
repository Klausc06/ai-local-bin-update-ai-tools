package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

type Profile struct {
	GOOS          string
	Home          string
	CodexHome     string
	ClaudeHome    string
	AgentsHome    string
	WorkBuddyHome string
	LaunchDirs    []string
	ConfigFiles   []string
	Notes         []string
}

func Detect(homeOverride string) Profile {
	home := homeOverride
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	p := Profile{
		GOOS:          runtime.GOOS,
		Home:          home,
		CodexHome:     filepath.Join(home, ".codex"),
		ClaudeHome:    filepath.Join(home, ".claude"),
		AgentsHome:    filepath.Join(home, ".agents"),
		WorkBuddyHome: filepath.Join(home, ".workbuddy"),
		ConfigFiles: []string{
			filepath.Join(home, ".codex", "config.toml"),
			filepath.Join(home, ".codex", "hooks.json"),
			filepath.Join(home, ".claude", "settings.json"),
			filepath.Join(home, ".claude", "settings.local.json"),
			filepath.Join(home, ".claude", ".mcp.json"),
			filepath.Join(home, ".workbuddy", ".mcp.json"),
			filepath.Join(home, ".workbuddy", "mcp.json"),
			filepath.Join(home, ".workbuddy", ".connectors-marketplace.meta.json"),
		},
	}
	switch runtime.GOOS {
	case "darwin":
		p.LaunchDirs = []string{filepath.Join(home, "Library", "LaunchAgents")}
		p.Notes = append(p.Notes, "macOS adapter enabled")
	case "linux":
		p.LaunchDirs = []string{
			filepath.Join(home, ".config", "systemd", "user"),
			filepath.Join(home, ".config", "autostart"),
			filepath.Join(home, ".local", "share", "applications"),
		}
		p.Notes = append(p.Notes, "linux adapter: systemd user services, XDG autostart, desktop entries; *.service files scanned via LaunchDirs")
	case "windows":
		p.LaunchDirs = []string{
			filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
			filepath.Join(home, "AppData", "Local", "Programs"),
		}
		p.Notes = append(p.Notes, "windows adapter: startup folder and local programs; Registry and Task Scheduler are manual review only")
	default:
		p.Notes = append(p.Notes, "unknown platform adapter")
	}
	return p
}
