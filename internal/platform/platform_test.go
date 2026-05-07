package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectDefaultHome(t *testing.T) {
	p := Detect("")
	if p.Home == "" {
		t.Fatal("expected non-empty home")
	}
	if p.GOOS != runtime.GOOS {
		t.Fatalf("expected GOOS %q, got %q", runtime.GOOS, p.GOOS)
	}
}

func TestDetectHomeOverride(t *testing.T) {
	p := Detect("/custom/home")
	if p.Home != "/custom/home" {
		t.Fatalf("expected /custom/home, got %q", p.Home)
	}
	if p.CodexHome != filepath.Join("/custom/home", ".codex") {
		t.Fatalf("unexpected CodexHome: %q", p.CodexHome)
	}
}

func TestDetectConfigFiles(t *testing.T) {
	p := Detect("/test")
	if len(p.ConfigFiles) == 0 {
		t.Fatal("expected non-empty ConfigFiles")
	}
	expected := []string{
		filepath.Join("/test", ".codex", "config.toml"),
		filepath.Join("/test", ".codex", "hooks.json"),
		filepath.Join("/test", ".claude", "settings.json"),
		filepath.Join("/test", ".claude", ".mcp.json"),
	}
	for _, want := range expected {
		found := false
		for _, cf := range p.ConfigFiles {
			if cf == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected ConfigFile %q not found", want)
		}
	}
}

func TestDetectToolHomes(t *testing.T) {
	p := Detect("/myhome")
	wantCodex := filepath.Join("/myhome", ".codex")
	wantClaude := filepath.Join("/myhome", ".claude")
	wantAgents := filepath.Join("/myhome", ".agents")
	wantWorkBuddy := filepath.Join("/myhome", ".workbuddy")
	if p.CodexHome != wantCodex {
		t.Fatalf("unexpected CodexHome: %q", p.CodexHome)
	}
	if p.ClaudeHome != wantClaude {
		t.Fatalf("unexpected ClaudeHome: %q", p.ClaudeHome)
	}
	if p.AgentsHome != wantAgents {
		t.Fatalf("unexpected AgentsHome: %q", p.AgentsHome)
	}
	if p.WorkBuddyHome != wantWorkBuddy {
		t.Fatalf("unexpected WorkBuddyHome: %q", p.WorkBuddyHome)
	}
}

func TestDetectLaunchDirsNotEmpty(t *testing.T) {
	p := Detect("/test")
	if len(p.LaunchDirs) == 0 {
		t.Fatal("expected non-empty LaunchDirs")
	}
	for _, dir := range p.LaunchDirs {
		if dir == "" {
			t.Fatal("LaunchDirs contains empty string")
		}
	}
}

func TestDetectNotesNotEmpty(t *testing.T) {
	p := Detect("/test")
	if len(p.Notes) == 0 {
		t.Fatal("expected non-empty Notes")
	}
}

func TestDetectDarwinLaunchDirs(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	p := Detect("/test")
	expected := filepath.Join("/test", "Library", "LaunchAgents")
	found := false
	for _, dir := range p.LaunchDirs {
		if dir == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LaunchDir %q not found in %v", expected, p.LaunchDirs)
	}
}

func TestDetectDarwinNotes(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	p := Detect("/test")
	found := false
	for _, note := range p.Notes {
		if note == "macOS adapter enabled" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'macOS adapter enabled' note, got %v", p.Notes)
	}
}
