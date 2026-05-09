# ai-local-bin-update-ai-tools — Project Context

## Active Branches
- `main` — primary development branch

## Current State
- 37 commits on `main`, pushed to origin
- Working tree clean (only `.cache/` untracked)
- `make install` done → `update-ai-tools 8f13ad9` in `~/.local/bin/`
- Go project at ~/Documents/Projects/ai-local-bin-update-ai-tools
- Remote: github.com/Klausc06/ai-local-bin-update-ai-tools

## Purpose
Conservative local updater for AI tooling. Inventories Codex, Claude, OMX, global skills, WorkBuddy, agents, and MCP config, then updates low-risk tools.

## Environment
- Go project, standard toolchain
- Run: `go test ./...`
- In sandboxed Codex, use repo-local `GOCACHE` if the default Go cache is blocked:
  `GOCACHE=$PWD/.cache/go-build go test ./...`

## Shared Memory
- Project memories are shared across Codex, fcc, and Claude Code via mem0 MCP
  and `.memory/` files. See `.memory/feedback.md` for the full three-layer
  architecture, metadata conventions, and dedup-before-write rules.
- Always verify memory claims against live git state, test results, and binary
  version before acting.
