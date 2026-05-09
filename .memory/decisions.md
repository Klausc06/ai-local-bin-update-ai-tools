# ai-local-bin-update-ai-tools — Decisions Log

## 2026-05-07: Platform adapter completion
- Completed Linux (XDG autostart, systemd) and Windows (startup folder) adapters
- Fixed ConfigFiles initialization order bug
- Removed dead init() function

## 2026-05-07: Test coverage push
- Added 42 tests across 5 packages
- app: 0% → 40%, provider: 0% → 35%, runner: 15% → 75%, report: 36% → 100%

## 2026-05-07/08: Color output + terminal detection
- Rewrote printHuman with ANSI color-coded output
- Added terminal auto-detection for color
- Fixed levelColor/statusGlyph to use colors struct

## 2026-05-08: Safety fixes from CodeBuddy review
- Non-action flags such as `--json`, `--verbose`, `--only`, and `--skip` must default to check mode and must not imply update.
- `--only` / `--skip` provider names are validated against the default provider registry.
- Update mode requires backup to complete before selected update tasks run.
- `--force` is only an override for partial backup warnings; hard backup failures still block updates.
- Selected update tasks that fail or are skipped because the executable is missing must make update mode return non-zero.
- Task reports include `resolved_path` for the executable found via `PATH`.

## 2026-05-08: Codex / Claude Code shared memory bridge
- This project uses file memory in `.memory/` plus mem0 project memories for cross-software continuity between Codex, fcc, and Claude Code.
- Current mem0 MCP filter syntax for this project is `{"AND":[{"metadata":{"project":"ai-local-bin-update-ai-tools"}}]}`.
- Memory entries are context hints, not proof; verify current branch, working tree, coverage, and test results with local commands.

## 2026-05-08: Session 7 safety fixes committed and installed
- Commit `155ee37` on `main`: hardened update safety per CodeBuddy review report
- Explicit action model, provider validation, backup gating, resolved_path, non-zero exits
- All 7 packages pass with -race, vet clean, coverage 64-96%
- Installed: `update-ai-tools 155ee37`

## 2026-05-08/09: Session 8 — Slim default output
- Commits `9b022e0`, `08f901a`, `932b709`, `d554947`, `a80f5fa`, `3429ea2` on `main`
- Default (non-verbose): summary bar, Actions, Warnings, actionable Risks, log/backup paths only; `--verbose` restores full Checks/Details/Advisory
- `Logger.Infof` console writes gated on verbose; `Logger.ProgressBar` always writes animated progress bar to console
- Warnings dedup: results already shown in Checks/Actions not repeated in Warnings
- MCP list table headers (Name...Status) compacted to "N servers"
- Noisy first lines ("Updating...", "Checking...") replaced with last meaningful output line; `\r` normalized to `\n`
- Filling progress bar `[████░░] N/M` with `\r` line overwrite in light green ANSI; `ProgressBar` + `ProgressDone` API
- Makefile `codesign --force --sign -` after install to prevent macOS kill of unsigned binaries
- `printRisksSectionBrief`, `compactSummary`, `lastSignificantLine`, `normalizeLines`, `ProgressBar`, `ProgressDone` helpers added; dead `Progressf` removed
- Coverage: app 88.7%, runner 82.3%, report 91.7%, all 7 packages pass with -race
- Installed: `update-ai-tools 3429ea2`
