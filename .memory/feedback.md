# ai-local-bin-update-ai-tools — User Feedback

## 2026-05-09: Cross-platform and generality principles

**Rule**: Every change must consider all platforms (macOS/Linux/Windows) and all user configurations. Write generic patterns, not single-case rules. Never make changes that only apply to one computer.

**Why**: The tool is distributed to other users with different setups. My-machine-only fixes don't scale.

**How to apply**:
- Before writing a pattern match, ask: does this catch the general case, or just my current output?
- Before changing behavior, ask: does this work on Linux/Windows too?
- Before running a one-off command (like `npm install -g`), ask: is this a code fix or a personal config tweak?
- After every change: PR, WORKLOG, .memory/*, mem0. No exceptions.

## 2026-05-09: Dedup before writing memory

**Rule**: Before writing to any memory (WORKLOG, .memory/*, mem0), check what's already there. Update/consolidate existing entries instead of appending duplicates.

**Why**: Session 8 produced 4 separate decisions.md entries and 6 separate mem0 entries for the same body of work. Each intermediate commit summary was written as a new entry instead of updating the prior one. The result is drift and noise — a future reader sees stale intermediate state and can't tell which is current.

**How to apply**:
- Before writing a new WORKLOG section, read the last section — if same session, update it
- Before writing a new decisions.md entry, check if the topic already has an entry — consolidate
- Before `add_memory` to mem0, search existing project memories — delete stale, update existing, or write one consolidated entry
- One session = one memory entry, not one per commit

## 2026-05-09: Three-layer memory architecture

**Rule**: Know which layer to write to.

**Layer 1 — mem0 shared**: Cross-tool, cross-session value. Preferences, conventions, verified environment state, project-level decisions, handoff. Search before writing (`search_memories` → update or add). Every entry must carry metadata:
- `author`: `claude-code` | `codex`
- `project`: `global` | `<project-name>`
- `category`: `convention` | `infrastructure` | `project_state` | `user_preferences` | `handoff`

Never store: secrets, temporary logs, unverified guesses, code structure trivia.

**Layer 2 — .memory/* files**: Project-level, human-readable, reviewable state. `context.md` (current branch/task/install state), `decisions.md` (what and why), `preferences.md` (code style, test targets), `feedback.md` (user corrections and rules).

**Layer 3 — local agent memory**: Codex/Claude Code internal long-term summaries. Not our concern, each tool manages its own.

**How to apply**:
- Cross-tool handoff fact → mem0 with full metadata
- Project state change (new commit, new coverage, new install) → update `.memory/context.md`, may also update mem0
- Design decision → `.memory/decisions.md`, may also write mem0 if cross-tool relevant
- User correction → `.memory/feedback.md`
- Before ANY memory write: search first, dedup, then write
