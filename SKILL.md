---
name: pp-notion
description: "CLI + MCP server for the Notion API. 31 typed tools, agent-first defaults."
author: "glbinv"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - notion-pp-cli
    install:
      - kind: go
        bins: [notion-pp-cli]
        module: github.com/glbinv/notion-cli/cmd/notion-pp-cli
---

# Notion — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `notion-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first (requires Go 1.26.3 or newer):

```bash
go install github.com/glbinv/notion-cli/cmd/notion-pp-cli@latest
```

Then:

1. Verify: `notion-pp-cli --version`
2. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Combined CLI for multiple API services

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**blocks** — Manage blocks

- `notion-pp-cli blocks delete` — Delete a block using its ID.
- `notion-pp-cli blocks retrieve` — Retrieve a block object using its ID.
- `notion-pp-cli blocks update` — Update a block's properties.

**children** — Manage children

- `notion-pp-cli children retrieve-block` — Retrieve the children of a block.
- `notion-pp-cli children update-block` — Update the children of a block.

**notion-search** — Manage notion search

- `notion-pp-cli notion-search` — Search for pages, databases, and other objects within a Notion workspace.

**pages** — Manage pages

- `notion-pp-cli pages create` — Create a new page in a Notion database.
- `notion-pp-cli pages retrieve` — Retrieve a page object using the page ID.
- `notion-pp-cli pages update` — Update the properties of a specific page by its ID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
notion-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `notion-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
notion-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `NOTION_BEARER_AUTH` as an environment variable.

Run `notion-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  notion-pp-cli notion-search --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
notion-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
notion-pp-cli feedback --stdin < notes.txt
notion-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/notion-pp-cli/feedback.jsonl`. They are never POSTed unless `NOTION_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NOTION_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
notion-pp-cli profile save briefing --json
notion-pp-cli --profile briefing notion-search
notion-pp-cli profile list --json
notion-pp-cli profile show briefing
notion-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `notion-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/glbinv/notion-cli/cmd/notion-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add notion-pp-mcp -- notion-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which notion-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   notion-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `notion-pp-cli <command> --help`.
