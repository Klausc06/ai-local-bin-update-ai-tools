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
	}
	switch runtime.GOOS {
	case "darwin":
		p.LaunchDirs = []string{filepath.Join(home, "Library", "LaunchAgents")}
		p.Notes = append(p.Notes, "macOS adapter enabled")
	case "linux":
		p.LaunchDirs = []string{filepath.Join(home, ".config", "systemd", "user")}
		p.Notes = append(p.Notes, "linux adapter stub: systemd user service detection reserved")
	case "windows":
		p.LaunchDirs = []string{filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "Startup")}
		p.Notes = append(p.Notes, "windows adapter stub: service and startup detection reserved")
	default:
		p.Notes = append(p.Notes, "unknown platform adapter")
	}
	p.ConfigFiles = []string{
		filepath.Join(p.CodexHome, "config.toml"),
		filepath.Join(p.CodexHome, "hooks.json"),
		filepath.Join(p.ClaudeHome, "settings.json"),
		filepath.Join(p.ClaudeHome, "settings.local.json"),
		filepath.Join(p.ClaudeHome, ".mcp.json"),
		filepath.Join(p.WorkBuddyHome, ".mcp.json"),
		filepath.Join(p.WorkBuddyHome, "mcp.json"),
		filepath.Join(p.WorkBuddyHome, ".connectors-marketplace.meta.json"),
	}
	return p
}
