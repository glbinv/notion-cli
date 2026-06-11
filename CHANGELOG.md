# Changelog

All notable changes to this project will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Markdown round-trip commands** (`format-write-page`, `format-update-block`, `format-read-page`) plus the internal `internal/notionmd` renderer (blocks ↔ extended markdown), exposed as named MCP tools so MCP hosts see them in `tools/list` instead of only the auto-generated cobra-mirror names.
- **Parity with the official Notion MCP** — 11 typed MCP tools covering REST endpoints the blocks-management spec omitted: databases (create/retrieve/update/query), comments (list/create), users (list/retrieve/me), `pages-move`, and `pages-duplicate` (compound copy of properties + full block tree).
- 31 typed MCP tools total: 10 endpoint + 3 framework + 7 magic-moment + 11 parity.

See [`.printing-press-patches.json`](./.printing-press-patches.json) for full rationale on each.

### Deferred upstream

- The hand-registered databases/comments/users tools should eventually be generated from a fuller Notion OpenAPI spec; a `/printing-press-reprint` against an expanded spec would supersede the parity patch.
