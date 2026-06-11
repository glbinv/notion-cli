# Security Policy

## Supported versions

Only the latest tagged release is supported. There is no LTS branch.

## Reporting a vulnerability

**Do not file a public GitHub issue for security reports.**

Use GitHub's [private vulnerability reporting](https://github.com/glbinv/notion-cli/security/advisories/new) on this repo. If that's unavailable, open an empty issue titled `Security report — please contact me privately` and wait for the maintainer to reach out.

When reporting, please include:

- A description of the issue and its impact (e.g., what an attacker could read/write/exfiltrate).
- A minimal reproduction — the command, input, and observed behavior.
- The version (`notion-pp-cli --version`) and OS / arch.
- Whether the issue reproduces against the [official Notion MCP server](https://github.com/makenotion/notion-mcp-server) (helps triage whether it's a Notion API issue vs. ours).

## What's in scope

- Token / credential handling in `notion-pp-cli` and `notion-pp-mcp`.
- Command injection, path traversal, or unsafe file writes.
- MCP tool surface — tools doing more than their description claims.
- Network behavior — unexpected outbound calls, leaked headers.

## What's out of scope

- Issues in the upstream Notion API itself — report those to Notion.
- Issues in [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) generator output that affect every printed CLI — please report there.
- Denial-of-service via expensive but documented operations (e.g., a full workspace sync).

## Disclosure timeline

We aim to acknowledge reports within 5 business days and patch confirmed issues within 30 days. Coordinated disclosure timing can be negotiated.
