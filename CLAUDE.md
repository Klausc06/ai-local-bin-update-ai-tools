# AGENTIC DIRECTIVE

## PROJECT

ai-local-bin-update-ai-tools — Conservative local updater for AI tooling (Codex, Claude, OMX, MCP, etc.). Go project.

## ENVIRONMENT
- Go standard toolchain
- Run tests: `go test ./...`
- Coverage: `go test -cover ./...`
- In sandboxed Codex, use repo-local `GOCACHE` if the default Go cache is blocked:
  `GOCACHE=$PWD/.cache/go-build go test ./...`

## IDENTITY
- Expert Go developer. Write idiomatic, simple Go code.
- Goal: Keep the tool conservative — never edit tokens, reset logins, or replace local MCP binaries.
- Every change must consider all platforms (macOS/Linux/Windows). Write generic patterns, not single-case rules.

## MEMORY SYSTEM

Three-layer architecture. Know which layer to write to before writing.

### Layer 1: File-system memory (.memory/)
- **On session start**: Read `.memory/context.md` `.memory/decisions.md` `.memory/preferences.md` `.memory/feedback.md`
- **On state change**: Update relevant `.memory/` files
- **On session end**: Update `.memory/context.md` with current state
- **Before writing**: Check for existing entries on the same topic — consolidate, don't duplicate

### Layer 2: Mem0 global memory (via MCP tools, if available)
- **On session start**: Search mem0 for project memories with `search_memories`
- **When to write**: Cross-project decisions, global preferences, verified project state for cross-tool handoff
- **Before writing**: `search_memories` first — if stale entries exist, delete them; if updatable, update; only add if truly new. One session = one consolidated entry, not one per commit.
- **Metadata required**: `author` (claude-code | codex), `project` (global | project name), `category` (convention | infrastructure | project_state | user_preferences | handoff)
- Never store: secrets, temporary logs, unverified guesses, code structure trivia

### Layer 3 — Local agent memory
- Each tool (Codex, Claude Code) manages its own internal summaries. Not our concern.

### After every change checklist (no exceptions):
1. WORKLOG.md updated and committed
2. `.memory/context.md` updated
3. `.memory/decisions.md` updated (if decision-level change)
4. mem0: search → delete stale → write one current entry
5. Push to origin

## CROSS-PLATFORM
- Use `filepath.Join` for paths, never hardcode `/` or `\`
- Test with `-race` on every change
- govulncheck before release
- Makefile `codesign --force --sign -` handles macOS unsigned binary kill
