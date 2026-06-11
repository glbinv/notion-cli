# notion-cli

A Go CLI and MCP server for the [Notion API](https://developers.notion.com/), built for use by humans **and** AI agents (Claude Code, Claude Desktop, Codex, Cursor, etc.).

- **31 MCP tools** covering the full public Notion REST surface — blocks, pages, databases, comments, users, search, plus higher-level write/sync/workflow tools.
- **Agent-first defaults** — non-interactive, JSON-piping, dry-run-able, with stable exit codes.
- **One repo, two binaries** — `notion-pp-cli` for the shell, `notion-pp-mcp` for [MCP](https://modelcontextprotocol.io/) hosts.

> **Built with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) by [Matt Van Horn](https://github.com/mvanhorn)** — a generator for ship-ready CLIs from an API spec. This repo is patterned after the published [printing-press-library](https://github.com/mvanhorn/printing-press-library); the parity-with-official-Notion-MCP work and the higher-level magic-moment tools are local extensions documented in [`.printing-press-patches.json`](./.printing-press-patches.json). See [NOTICE](./NOTICE).

## Install

### Go (recommended, requires Go 1.26.3+)

```bash
go install github.com/glbinv/notion-cli/cmd/notion-pp-cli@latest
go install github.com/glbinv/notion-cli/cmd/notion-pp-mcp@latest
```

### Pre-built binary

Download a release archive for your platform from the [latest release](https://github.com/glbinv/notion-cli/releases/latest).

```bash
# macOS — clear the Gatekeeper quarantine after extracting
xattr -d com.apple.quarantine notion-pp-cli notion-pp-mcp

# Unix — mark executable
chmod +x notion-pp-cli notion-pp-mcp
```

### Build from source

```bash
git clone https://github.com/glbinv/notion-cli.git
cd notion-cli
make build-all   # produces ./bin/notion-pp-cli and ./bin/notion-pp-mcp
```

## Use with Claude Desktop (MCP)

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle for one-click install in Claude Desktop ≥ 1.0.0.

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/glbinv/notion-cli/releases/latest).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Paste your `NOTION_BEARER_AUTH` token when prompted.

Pre-built bundles ship for `darwin-arm64` and Windows (`amd64`, `arm64`). For other platforms, use the manual JSON config below.

<details>
<summary>Manual JSON config (advanced)</summary>

```bash
go install github.com/glbinv/notion-cli/cmd/notion-pp-mcp@latest
```

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "notion": {
      "command": "notion-pp-mcp",
      "env": {
        "NOTION_BEARER_AUTH": "<your-token>"
      }
    }
  }
}
```

</details>

## Quick start

### 1. Get a token

Create an integration at <https://www.notion.so/profile/integrations>, then share the pages/databases you want to access with the integration.

### 2. Configure

```bash
notion-pp-cli auth set-token YOUR_TOKEN
# or
export NOTION_BEARER_AUTH="YOUR_TOKEN"
```

### 3. Verify

```bash
notion-pp-cli doctor
```

### 4. Try it

```bash
notion-pp-cli notion-search --query "meeting notes" --json
```

## vs. the official Notion MCP server

The [official Notion MCP server](https://github.com/makenotion/notion-mcp-server) wraps the same public REST API. This CLI covers the same surface and adds higher-level, agent-friendly commands on top.

| Capability | This CLI (`notion-pp-mcp`) | Official `@notionhq/notion-mcp-server` |
| --- | --- | --- |
| Search workspace | ✅ `notion-search`, `search` | ✅ |
| Blocks CRUD | ✅ retrieve / update / delete / children | ✅ |
| Pages CRUD | ✅ create / retrieve / update / move / duplicate | ✅ (no first-class duplicate) |
| Databases CRUD | ✅ create / retrieve / update / query | ✅ |
| Comments | ✅ list / create | ✅ |
| Users | ✅ list / retrieve / me | ✅ |
| Markdown round-trip | ✅ `format-write-page`, `format-update-block`, `format-read-page` | ❌ |
| Local sync to SQLite | ✅ `sync-database`, `sync-page`, `search-local` (offline-friendly) | ❌ |
| Workflow runner | ✅ `workflow-run` (named multi-step recipes) | ❌ |
| Distribution | Single Go binary + MCPB bundle | Node.js package |
| Transport | Native MCP **and** Cobra CLI from one codebase | MCP server only |

Tool taxonomy (31 typed MCP tools):

- **10 endpoint tools** — generated 1:1 from the Notion OpenAPI spec (blocks, pages, search, children).
- **3 framework tools** — `search`, `sql`, `agent-context`.
- **7 magic-moment tools** — `format-write-page`, `format-update-block`, `format-read-page`, `sync-database`, `sync-page`, `search-local`, `workflow-run`.
- **11 parity tools** — `databases-*`, `comments-*`, `users-*`, `pages-move`, `pages-duplicate` (added to close the gap with the official MCP).

Full rationale: [`.printing-press-patches.json`](./.printing-press-patches.json).

## Commands

Run `notion-pp-cli --help` for the live tree. Highlights:

### blocks

- `notion-pp-cli blocks delete BLOCK_ID` — delete a block.
- `notion-pp-cli blocks retrieve BLOCK_ID` — retrieve a block object.
- `notion-pp-cli blocks update BLOCK_ID --type ...` — update a block's properties.

### children

- `notion-pp-cli children retrieve-block BLOCK_ID` — retrieve a block's children.
- `notion-pp-cli children update-block BLOCK_ID --children ...` — update a block's children.

### pages

- `notion-pp-cli pages create --parent ... --properties ...` — create a new page.
- `notion-pp-cli pages retrieve PAGE_ID` — retrieve a page object.
- `notion-pp-cli pages update PAGE_ID --properties ...` — update page properties.

### notion-search

- `notion-pp-cli notion-search --query "..."` — search across pages, databases, and other objects.

## Examples

End-to-end walkthroughs. Each one is annotated; copy a block, swap the IDs, and it runs.

### Find a page, read it, update one property

```bash
# 1. Search the workspace. --select keeps the response small.
notion-pp-cli notion-search \
  --query "Q3 planning" \
  --json --select results.id,results.url,results.properties.Name

# 2. Use the page ID from step 1 to read the page object.
notion-pp-cli pages retrieve 11111111-2222-3333-4444-555555555555 --json

# 3. Update one property (Status -> Done). --dry-run prints the request first.
notion-pp-cli pages update 11111111-2222-3333-4444-555555555555 \
  --properties '{"Status":{"select":{"name":"Done"}}}' \
  --dry-run

# 4. Drop --dry-run to actually send it. --agent gives JSON + no prompts.
notion-pp-cli pages update 11111111-2222-3333-4444-555555555555 \
  --properties '{"Status":{"select":{"name":"Done"}}}' \
  --agent
```

### Write a page from Markdown, then read it back

The `format-*` magic-moment commands round-trip Notion blocks ↔ extended markdown, so you can author and edit pages as plain text.

```bash
# Write a new page body from Markdown
cat > meeting.md <<'EOF'
# Standup — 2026-06-07

## Decisions
- Ship CLI v0.1 by EOW
- Defer parity tools v2 to next sprint

## Action items
- [ ] glbinv: open the public repo
- [ ] reviewer: code-review the parity patch
EOF

notion-pp-cli format write-page \
  --page-id 11111111-2222-3333-4444-555555555555 \
  --md meeting.md \
  --mode replace

# Read it back as Markdown to verify
notion-pp-cli format read-page 11111111-2222-3333-4444-555555555555
```

### Sync a database locally, then query offline

`sync-*` materializes Notion content into a local SQLite store. `search-local` queries it without hitting the API — useful when an agent needs to iterate quickly or when you're offline.

```bash
# Pull the database (and all child pages) into the local store
notion-pp-cli sync database DATABASE_ID --full

# Query the local store — no API call, no rate limit
notion-pp-cli search-local --query "action item" --limit 20 --json

# Incremental refresh later
notion-pp-cli sync database DATABASE_ID --since 2026-06-01
```

### Use from Claude Desktop (MCP)

Once the MCPB bundle is installed (see [Use with Claude Desktop](#use-with-claude-desktop-mcp) above), the 31 typed tools become callable in any Claude Desktop conversation. Example prompts that route to specific tools:

| Prompt | Tool that runs |
| --- | --- |
| "Find pages tagged Q3 planning" | `notion-search` |
| "What's on page X?" | `pages-retrieve` → `format-read-page` |
| "Add a comment to this page" | `comments-create` |
| "Who's in the workspace?" | `users-list` |
| "Duplicate the project template into Q3" | `pages-duplicate` |
| "Move this page under Roadmap" | `pages-move` |

## Output formats and agent flags

```bash
# Human-readable table (default when stdout is a TTY; JSON when piped)
notion-pp-cli notion-search --query "Q2 planning"

# Force JSON (for scripts and agents)
notion-pp-cli notion-search --query "Q2 planning" --json

# Filter to specific fields
notion-pp-cli notion-search --query "Q2 planning" --json --select id,title,url

# Dry run — show the prepared request without sending it
notion-pp-cli pages update PAGE_ID --properties '{...}' --dry-run

# Agent mode — JSON + compact output + non-interactive + no color, in one flag
notion-pp-cli notion-search --query "Q2 planning" --agent
```

## Agent usage

Designed for AI-agent consumption:

- **Non-interactive** — every input is a flag; no prompts.
- **Pipeable** — `--json` to stdout, errors to stderr.
- **Filterable** — `--select id,name` returns only the fields you ask for.
- **Previewable** — `--dry-run` shows the request without sending.
- **Explicit retries** — `--idempotent` on creates and `--ignore-missing` on deletes make a no-op success acceptable.
- **Confirmable** — `--yes` explicitly confirms destructive actions.
- **Piped input** — write commands accept structured input via `--stdin` (see the command's `--help`).
- **Offline-friendly** — `sync-*` commands materialize Notion content to a local SQLite store; `search-local` queries it.
- **Agent-safe by default** — no colors or pretty formatting unless `--human-friendly` is set.

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health check

```bash
notion-pp-cli doctor          # checks config + creds + connectivity
notion-pp-cli doctor --json   # machine-readable
```

## Configuration

Config file: `~/.config/notion-pp-cli/config.toml`

Static request headers can be set under `[headers]`; per-command `--header` overrides take precedence.

| Env var | Required | Description |
| --- | --- | --- |
| `NOTION_BEARER_AUTH` | yes | Your Notion integration token. |

## Troubleshooting

**Authentication errors (exit code 4)**

```bash
notion-pp-cli doctor
echo $NOTION_BEARER_AUTH   # confirm the env var is set
```

**Not found errors (exit code 3)** — verify the resource ID and confirm the integration has been shared into the page/database.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). Local extensions to the generated tree are tracked in [`.printing-press-patches.json`](./.printing-press-patches.json); the schema is documented in [AGENTS.md](./AGENTS.md).

## Credits

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) ([Matt Van Horn](https://github.com/mvanhorn)) and patterned after the [printing-press-library](https://github.com/mvanhorn/printing-press-library). Local extensions (parity tools, markdown round-trip, local sync) are documented in [`.printing-press-patches.json`](./.printing-press-patches.json). See [NOTICE](./NOTICE) for full attribution.

## License

[Apache 2.0](./LICENSE).
