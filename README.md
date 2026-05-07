# update-ai-tools

`update-ai-tools` is a conservative local updater for AI tooling on this machine.

It inventories Codex, Claude, OMX, global skills, WorkBuddy, agents, and MCP
configuration, then updates only the low-risk tools with explicit update
commands. It never edits tokens, resets logins, or replaces local third-party
MCP binaries.

## Usage

```bash
update-ai-tools --check
update-ai-tools --dry-run
update-ai-tools --menu
update-ai-tools
update-ai-tools --check --json
update-ai-tools --version
```

## Modes

- `--check`: inventory only. It writes a redacted log but does not back up or
  update configs.
- `--dry-run`: inventory plus planned update commands. It does not back up or
  update configs.
- `--menu`: opens an interactive terminal menu.
- default: backs up known config files, runs safe update commands, then performs
  post-update checks.
- `--json`: prints a machine-readable report for future WorkBuddy or frontend
  integration.
- `--version`: prints the installed command version.
- `--only codex,omx` / `--skip skills`: narrow the provider set while debugging.

Logs are written to:

```text
~/.codex/log/update-ai-tools/YYYYMMDD-HHMMSS.log
```

Backups from update mode are written to:

```text
~/.codex/backups/update-ai-tools/YYYYMMDD-HHMMSS/
```

## Safe Updates

The default update mode runs:

- `codex update`
- `claude update`, with `claude upgrade` as a compatibility fallback
- `omx update`
- `npx skills update -g -y`

Each command is isolated. A failure in one update does not stop the rest.

## Manual Review Items

These are reported but never automatically updated:

- local Xiaohongshu MCP services or LaunchAgents
- hand-written Spotify MCP services
- configs containing token, secret, API key, auth, bearer, password, or phone-like fields
- WorkBuddy marketplace/cache content
- unknown local MCP binaries or services

## Development

```bash
make test
make build
make release
```

The first complete implementation targets macOS. Linux and Windows platform
paths are stubbed in the platform adapter so the core stays portable.

`make release` writes reproducible local binaries to `dist/`:

- `update-ai-tools-darwin-arm64`
- `update-ai-tools-darwin-amd64`
- `update-ai-tools-linux-arm64`
- `update-ai-tools-linux-amd64`
- `update-ai-tools-windows-amd64.exe`

For local installation from a checkout:

```bash
make install
```
