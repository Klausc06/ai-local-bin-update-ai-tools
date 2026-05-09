# Work Log

## Project: update-ai-tools

A conservative local CLI updater for AI development tooling (Codex, Claude, OMX, global skills, WorkBuddy, MCP configuration). Zero external dependencies, pure Go stdlib.

---

## Session 1 — Initial scaffolding

**Commit:** `fa76b22` — Create a conservative local AI toolchain updater

16 files, 1,283 lines. Full project skeleton in one pass:
- `cmd/update-ai-tools/main.go` — entry point
- `internal/app/` — CLI orchestration, flag parsing (--check, --dry-run, --json, --version, --only, --skip)
- `internal/backup/` — config file backup to timestamped directories with O_EXCL
- `internal/platform/` — macOS path detection (CodexHome, ClaudeHome, AgentsHome, WorkBuddyHome)
- `internal/provider/` — 6 providers (codex, claude, omx, skills, workbuddy, mcp) with inventory/update/post-check
- `internal/redactor/` — credential redaction (sk- keys, Bearer tokens, named secrets, URL params, phone numbers)
- `internal/report/` — data types, JSON serialization, logging
- `internal/runner/` — external command execution with timeout, fallback, health checking
- `scripts/` — bash and PowerShell wrappers
- README.md, go.mod (Go 1.22)

Minimal tests: 2 redactor tests, 1 report test, 8 runner tests.

---

## Session 2 — Platform adapters and test infrastructure

### `1be6c43` — feat: complete Linux and Windows platform adapters

Added `internal/platform/platform_test.go` (122 lines, 8 tests), `Makefile` (test/build/release/install/clean), `scripts/build-release.sh` (cross-compile to 5 targets). Extended `platform.Detect()` with Linux systemd/XDG paths and Windows Startup/Programs paths. Updated `.gitignore` and README.

### `a5fa2d6` — fix: remove dead init() and add tests for app, backup, provider

5 files, 614 lines. Added 42 tests across 3 packages:
- `app_test.go` (14 tests): flag parsing, provider filtering, warning results, JSON report
- `backup_test.go` (6 tests): safeName, copyFile with O_EXCL, Configs flow
- `provider_test.go` (18 tests): config risk scanning, MCP classification, file counting, JSON parsing

Removed dead `init()` from `platform.go` and `provider.go`.

### `71e1592` — test: add runner RunTask/Capture and report Logger/Summarize tests

2 files, 171 lines. Runner: 5 new tests (RunTask success/fail/skip/empty, Capture). Report: 6 new tests (Logger at all verbosity levels, Summarize, DeduplicateRisks).

**Milestone:** 63 tests total, all 7 internal packages have test coverage.

---

## Session 3 — Code audit and hardening

### `3b0aae6` — fix: harden error handling, precompile regexes, and add provider inventory tests

10 files, +345/-33. Full codebase audit followed by targeted fixes:

**Bug fixes (P0):**
- Guard against empty Home directory in `app.Run()` — fail with clear error instead of writing to filesystem root
- Lowercase usage error string per Go convention (ST1005)

**Behavior fixes (P1):**
- Distinguish zero-copied vs partial backup failures in `backup.Configs()`
- Remove over-broad "✗" match in `looksLikeHealthWarning` — only match "failed to connect"

**Code quality (P2):**
- Precompile 3 hot-path regexes (bearerRe, urlParamRe, urlExtractRe) as Redactor struct fields
- Stop discarding `filepath.Glob` error in mcpInventory launch-dir scan
- Log write errors to console instead of silently discarding
- Report JSON marshal failures to stderr
- Preserve Windows drive letter in `safeName`
- Support release binaries with arch suffix in bash wrapper

**Test additions:**
- `firstSignificantLine` edge cases (empty input, all-warning lines)
- `looksLikeHealthWarning` negative cases
- Redactor: Bearer token and env-file format tests
- Provider: dirItem, countFilesItem, countSkillItem, skillsInventory, workbuddyInventory tests
- Introduced `TaskRunner` interface for testable provider inventory
- 4 new provider inventory tests (codex/claude/omx) using stubRunner

**Milestone:** 77 tests, staticcheck clean, all audit findings resolved.

---

## Session 4 — CI, versioning, and e2e tests

### `fb8031a` — feat: align ps1 wrapper with release binary detection and add CI

- PowerShell wrapper: search both dev and release binaries, matching bash wrapper behavior
- `.github/workflows/ci.yml`: Ubuntu + macOS matrix, go vet + staticcheck + go test + go build

### `aa97b4e` — feat: inject git version, add -race, and add e2e/mcp inventory tests

- Version: changed from `const` to `var` in `app.go`, inject via `-ldflags -X` with `git describe --tags --always --dirty`
- Makefile: `VERSION` / `LDFLAGS` variables, `build-release.sh` passes them through
- `make test` now runs with `-race`
- e2e tests: `TestRunVersion`, `TestRunCheckJson`, `TestRunCheckAndDryRunExclusive` using stdout capture
- mcpInventory tests: full config scan + LaunchAgent glob flow, missing-config edge case
- govulncheck: zero vulnerabilities

**Milestone:** 81 tests, CI configured, release builds carry embedded git version.

---

## Session 5 — UX improvements

### `bd826b3` — feat: add interactive menu when run without args in terminal

No-args terminal invocation shows a numbered menu:
```
[1] Check  [2] Dry run  [3] Update  [4] JSON  [5] Version
```
Falls back to traditional flag behavior when stdin is not a terminal (scripts, pipes). All existing flags still work for automation.

### `e64b001` — fix: clean up human output formatting and strip ANSI escapes

Rewrote `printHuman()`:
- Auto-calculated column widths (capped at 28 chars)
- Long summaries truncated at 60 chars
- Results sorted by provider then name
- Risk dedup key excludes Provider (fixes duplicate mcp-output warnings)
- Warning and risk counts shown as section headers
- ANSI escape sequences stripped in redactor pipeline

**Milestone:** Polished terminal output, zero user friction.

---

## Session 6 — Polish and cross-platform hardening (2026-05-07)

### `fdcef95` — fix: use filepath.Join in TestDetectToolHomes for Windows compat

Replaced hardcoded `/` path separators with `filepath.Join` in platform tests so the test suite passes on Windows.

### `477888b` — fix: add MIT LICENSE, graceful TTY fallback for non-interactive environments

- Added MIT LICENSE file for open-source distribution
- `defaultArgs()` now falls back to `--check` when stdin is not a terminal (CI, scripts, pipes), preventing the interactive menu from blocking in automated workflows

### `298ba63` — feat: color-coded output with cleaner section layout

Rewrote `printHuman()` with full ANSI color support:
- Green checkmarks / red X / yellow skip-warning glyphs, dim metadata
- Top summary bar shows success/fail/skipped/warning counts at a glance
- Results auto-sorted into Checks, Actions, and Details sections
- Risks split into actionable Risks vs. informational Advisory groups
- `detectColors()` auto-disables color when output is not a terminal (pipes, redirects)

### `0b720e1` — fix: use colors-aware functions in levelColor and statusGlyph

`levelColor()` and `statusGlyph()` now delegate to the `colors` struct's color functions rather than hardcoded raw strings, ensuring consistent color behavior across all output elements.

**Milestone:** Cross-platform test suite, MIT-licensed, polished color output with clean section layout.

---

## Session 7 — Review of improvement report (2026-05-08)

Reviewed CodeBuddy's `update-ai-tools-improvement-report.md` against the live repo.

Key findings:
- The report's P0 about explicit non-action flags is valid: `update-ai-tools --json`, `--verbose`, `--only ...`, or `--skip ...` can still run in update mode because only zero-argument invocations go through `defaultArgs()`.
- The report's P0 about backup failure is valid: `backup.Configs()` records failed or warning status but the update loop continues unconditionally.
- Provider allowlist validation, resolved command path recording, and failure exit-code policy are the most practical next P1 fixes.
- Health/check separation is useful but should follow the safety fixes because current behavior matches the existing "full coverage, conservative, manual trigger" direction.
- Provider file splitting and plugin-style providers are longer-term refactors, not immediate safety work.

Recommended implementation order:
1. Make action explicit so only `--update` or menu Update can mutate.
2. Block updates on failed backup, with an explicit force override only if needed.
3. Validate `--only` / `--skip` provider names.
4. Add resolved command paths and non-zero exit policy for failed non-interactive updates.

No code changes were made in this review pass.

### Implementation follow-up — safety fixes from the report

Implemented the immediate P0/P1 safety items:
- Added an explicit action model so non-action flags default to check mode. `--json`, `--verbose`, `--only`, and `--skip` no longer imply update behavior.
- Added provider-name validation for `--only` and `--skip`; unknown names now fail early with the valid provider list.
- Made update mode require a clean backup before running update tasks. `--force` is the explicit override for partial backup warnings; hard backup failures still block updates.
- Added non-zero update-mode errors when selected update tasks fail or are skipped because their executable is missing, while still allowing independent tasks to run and be reported.
- Added `resolved_path` to task results so reports show the actual executable path found by `PATH`.
- Updated README behavior notes for `--force`, non-action `--json`, provider validation, backup gating, and update failure exit behavior.

Regression tests added/updated:
- parser defaults for non-action flags
- JSON check mode not backing up or updating
- provider filter validation for `--only` / `--skip`
- backup failure/warning blocking update execution
- `--force` continuing after partial backup warnings
- missing selected update executables returning non-zero
- failed update task returning an error
- runner resolved command path recording

Verification:
```text
GOCACHE=/Users/klaus/Documents/Projects/ai-local-bin-update-ai-tools/.cache/go-build go test ./...
```

Result: all packages passed. The first test run without local `GOCACHE` was blocked by sandbox access to `/Users/klaus/Library/Caches/go-build`, so verification used a repo-local Go build cache.

Code-review follow-up:
- Strengthened backup warning/failure tests so they assert the returned error, JSON report content, and skipped update result.
- Changed backup behavior so an existing config set where every copy fails is `failed`, not `warning`; `--force` only continues after partial backup warnings.
- Made selected update tasks that resolve to `skipped` return a non-zero update-mode error.
- Updated the drifted worklog summary counts.

---

## Summary

| Metric | Value |
|--------|-------|
| Total commits | 34 |
| Files | 22 source files + CI + scripts |
| Go packages | 7 internal + 1 cmd |
| Test functions | 138 |
| External dependencies | 0 |
| CI platforms | ubuntu + macos |
| Release targets | darwin/arm64, darwin/amd64, linux/arm64, linux/amd64, windows/amd64 |

---

## Session 8 — Slim default output (2026-05-08)

### `9b022e0` — feat: slim default output — verbose gates INFO console and detail sections

Simplified default human output so the terminal is not cluttered with routine
check results:

- Default (non-verbose): only the summary bar, Actions, Warnings, actionable
  Risks, and log/backup paths are displayed.
- Checks, Details, and Advisory sections are hidden unless `--verbose` is passed.
- `Logger.Infof` console writes are now gated on the verbose flag; the log file
  always receives every message regardless of verbosity.
- Added `printRisksSectionBrief` for non-verbose risk display (actionable risks
  only, no Advisory group).
- Added 6 tests: verbose/non-verbose printHuman section gating, empty section
  suppression, verbose/non-verbose console INFO gating, Logger.Infof verbose
  honor.

### `08f901a` — fix: dedup warnings and compact MCP list summaries

- Warnings already shown in Checks or Actions sections are no longer duplicated
  in the Warnings section.
- MCP list outputs with table headers (Name...Status columns) are compacted to
  "N servers" instead of raw column headers.
- Added `compactSummary` helper in runner with 4 new tests.
- Updated app tests for warning dedup behavior.

Tests: 130 passing, app 88.3%, runner 81.1%, all 7 packages with -race.

### `932b709` — fix: compact noisy update summaries — skip progress lines, normalize \r

- Update commands often print progress/spinner lines before the result.
  `compactSummary` now detects noisy first lines ("Updating...", "Checking...")
  and falls back to the last meaningful output line.
- Carriage returns (`\r`) are normalized to newlines via `normalizeLines` so
  progress-overwrite output (skills) is parsed correctly.
- Added `lastSignificantLine`, `normalizeLines` helpers and 8 new runner tests.
- `skills` npm package installed globally to eliminate `npm warn exec` noise.

Tests: 136 passing, runner 82.3%, all 7 packages with -race.

### `d554947` — feat: animated progress bar during update mode

- Replaced static step list with a filling progress bar that redraws on the same
  line using `\r`. Format: `[████░░░░░░] N/M Updating codex...`.
- `Logger.ProgressBar(step, total, label)` + `Logger.ProgressDone()` API.
- `ProgressDone` flushes the final newline after bar completes.
- Added `TestLoggerProgressBarAlwaysConsole`, `TestLoggerProgressBarDone`.

### `f084a8f` — fix: ad-hoc codesign after install to prevent macOS kill

- macOS kills unsigned binaries on execution. Added `codesign --force --sign -`
  to `make install` to ad-hoc sign the installed binary.

### `a80f5fa` — feat: light green progress bar color

- Wrapped progress bar line in `\033[92m` (bright green) ANSI code.

### `3429ea2` — chore: remove dead Progressf method

- Replaced by ProgressBar in d554947, no remaining call sites.

### `c9fffb2` — docs: align memory files with shared conventions

- `.memory/*` and `CLAUDE.md` tracked in git for the first time.
- Reading order codified: mem0 → .memory/* → CLAUDE.md → WORKLOG → git diff.
- After-change checklist: WORKLOG → context.md → decisions.md → mem0 → push.
- mem0 metadata rules (author/category), dedup-before-write, three-layer architecture.
- Aligned with `~/free-claude-code/.memory/conventions.md` cross-agent standards.

Tests: 138 passing, app 88.7%, runner 82.3%, report 91.7%, all 7 packages with -race.

---

## Session 9 — Summary normalization

### normalizeSummary: unify update tool status lines

Problem: The 4 update tools produce inconsistent status lines:
- `claude-update` → `Current version: 2.1.138` (plain)
- `codex-update` → `🎉 Update ran successfully! Please restart Codex.` (emoji, verbose)
- `omx-update` → `[omx] oh-my-codex is already up to date (v0.16.3).` (bracketed prefix)
- `skills-update-global` → `✓ All global skills are up to date` (checkmark)

Added `normalizeSummary` in `internal/runner/runner.go`:
- Strips leading emoji/unicode symbols (🎉, ✓, ✗, ⚠, etc.)
- Strips `[bracketed]` tool prefixes
- Compacts verbose phrases: ` already up to date` → ` up to date`, `Update ran successfully!` → `updated`
- Truncates `updated` with trailing fluff (e.g. `Please restart Codex.`)
- Collapses whitespace

Tests: 141 passing (3 new: strips emoji, strips bracketed prefix, passthrough).
Coverage: runner 82.9%, all 7 packages pass.

---

## Session 10 — Windows CI fix + staticcheck cleanup (Hermes)

### Root cause: Windows test failures

6 CI runs failed on `windows-latest` only (ubuntu/macOS green). Two tests in `internal/app/app_test.go`:

- `TestRunUpdateReturnsErrorWhenTaskFails`
- `TestRunUpdateForceContinuesAfterPartialBackupWarning`

Root cause: `installFakeNpx` helper created a `#!/bin/sh` script named `npx` (no extension).
On Windows, `exec.LookPath("npx")` cannot find a file without `.exe`/`.cmd`/`.bat` extension,
and there is no `/bin/sh` shell to execute it anyway. The skills provider's `SkipIfMissing: "npx"`
then silently skipped the task instead of letting it fail.

**Fix (commit `7438c77`, by fcc):** Platform-aware `installFakeNpx` using `runtime.GOOS`:
- Windows: creates `npx.cmd` batch file (`@echo off` + `exit /b N`)
- Unix: creates `npx` shell script (`#!/bin/sh`)
- Replaced single-digit exit code hack with `strconv.Itoa`

### `9e03c6c` — ci: add workflow_dispatch trigger

Added `workflow_dispatch` to `.github/workflows/ci.yml` so manual CI runs can be triggered
without pushing a commit.

### `b71d764` — fix: remove duplicate ⚠ from TrimLeft cutset (SA1024)

`staticcheck@latest` (v0.7.0) flagged `internal/runner/runner.go:216`: the cutset string
in `normalizeSummary` contained both `⚠` (U+26A0) and `⚠️` (U+26A0+U+FE0F), duplicating
the base code point. Removed standalone `⚠`, kept `⚠️` (emoji presentation form).

Tests: 141 passing, all 7 packages pass with -race on macOS.

