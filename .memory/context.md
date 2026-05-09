# ai-local-bin-update-ai-tools — Project Context

## Active Branches
- `main` — primary development branch

## Current State
- 39 commits on `main`, pushed to origin
- Working tree clean (only `.cache/` untracked)
- `make install` done → `update-ai-tools` in `~/.local/bin/`
- Go project at ~/Documents/Projects/ai-local-bin-update-ai-tools
- Remote: github.com/Klausc06/ai-local-bin-update-ai-tools
- CI: ubuntu ✅ macOS ✅ Windows ✅ (all 3 platforms green as of 2026-05-10)
- 141 tests, all 7 packages pass with -race
- Coverage: app 88.7%, backup 88.9%, platform 64.3%, provider 86.3%, redactor 85.7%, report 94.3%, runner 82.9%

## Purpose
Conservative local updater for AI tooling. Inventories Codex, Claude, OMX, global skills, WorkBuddy, agents, and MCP config, then updates low-risk tools.

## Environment
- Go project, standard toolchain
- Run: `go test ./...`
- In sandboxed Codex, use repo-local `GOCACHE` if the default Go cache is blocked:
  `GOCACHE=$PWD/.cache/go-build go test ./...`

## Shared Memory
- Project memories are shared across Codex, fcc, Claude Code, and Hermes via mem0 MCP
  and `.memory/` files. See `.memory/feedback.md` for the full three-layer
  architecture, metadata conventions, and dedup-before-write rules.
- Always verify memory claims against live git state, test results, and binary
  version before acting.

## Known Issues / Improvements
- Platform coverage 64.3% is inherently limited (OS branches can't all be tested on one OS)
- CI missing govulncheck (CLAUDE.md mentions it but not in ci.yml)
- CI uses Node 20 actions (checkout@v4, setup-go@v5) — upgrade before Sept 2026 deprecation
