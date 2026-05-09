# ai-local-bin-update-ai-tools — Project Preferences

## Code Style
- Go standard conventions
- Use filepath.Join for cross-platform paths
- No dead code

## Testing
- Aim for 70%+ coverage on all packages
- Integration tests preferred for full-pipeline coverage

## Agent Workflow
- **Reading order (every session)**: mem0 search → `.memory/*` (context, decisions, preferences, feedback) → `CLAUDE.md` → `WORKLOG.md` → `git diff`
- Search mem0 for project memories when work depends on prior cross-software context
- Shared cross-agent conventions: `~/free-claude-code/.memory/conventions.md`
- Write stable project decisions and cross-tool workflow facts back to shared memory so Codex and Claude Code do not require the user to repeat them
- One session = one consolidated mem0 entry, not one per commit. Search before writing.
- Keep transient command output out of memory unless it changes a reusable project decision

## After Every Change
1. WORKLOG.md updated and committed
2. `.memory/context.md` updated
3. `.memory/decisions.md` updated (if decision-level change)
4. mem0: search → delete stale → write one current entry with author/category metadata
5. Push to origin

## Cross-Agent Handoff
- The user often switches the same project between Codex, fcc, and Claude Code. Agents must leave enough shared context for another tool to continue without the user restating the task.
- Before taking over from another agent, read `mem0` → `.memory/*` → `CLAUDE.md` → `WORKLOG.md` → `git status --short` → `git diff --stat`.
