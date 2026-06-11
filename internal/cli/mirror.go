// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// mirror.go is hand-authored (NOT generated). It implements the annotation-aware
// local mirror + SQL query pair: `format mirror` walks a page's block tree into
// SQLite with per-run color/annotation columns, and `format sql` runs compound
// read-only queries over it. The generated `sync` command is a no-op for Notion
// because the spec exposes only by-ID endpoints (no list endpoints to crawl).

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

const mirrorSchema = `
CREATE TABLE IF NOT EXISTS pages (
  id TEXT PRIMARY KEY, title TEXT, url TEXT,
  created_time TEXT, last_edited_time TEXT, synced_at TEXT
);
CREATE TABLE IF NOT EXISTS blocks (
  id TEXT PRIMARY KEY, page_id TEXT, parent_id TEXT, type TEXT,
  position INTEGER, depth INTEGER, color TEXT, plain_text TEXT,
  has_children INTEGER, last_edited_time TEXT
);
CREATE TABLE IF NOT EXISTS rich_text (
  id INTEGER PRIMARY KEY AUTOINCREMENT, block_id TEXT, page_id TEXT,
  position INTEGER, content TEXT, color TEXT,
  bold INTEGER, italic INTEGER, strikethrough INTEGER, underline INTEGER, code INTEGER, href TEXT
);
CREATE INDEX IF NOT EXISTS idx_blocks_page ON blocks(page_id);
CREATE INDEX IF NOT EXISTS idx_blocks_type ON blocks(type);
CREATE INDEX IF NOT EXISTS idx_rt_block ON rich_text(block_id);
CREATE INDEX IF NOT EXISTS idx_rt_color ON rich_text(color);
CREATE VIEW IF NOT EXISTS colored_text AS
  SELECT rt.page_id, p.title AS page_title, rt.block_id, b.type AS block_type, rt.content, rt.color
  FROM rich_text rt JOIN blocks b ON b.id = rt.block_id JOIN pages p ON p.id = rt.page_id
  WHERE rt.color != 'default';
CREATE VIEW IF NOT EXISTS block_palette AS
  SELECT page_id, type, color, COUNT(*) AS n FROM blocks
  WHERE color IS NOT NULL AND color != 'default' GROUP BY page_id, type, color;
`

func defaultMirrorPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "notion-pp-cli", "mirror.db")
}

// --- format mirror ---------------------------------------------------------

func newFormatMirrorCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "mirror <page_id>",
		Short:   "Mirror a page's block tree into local SQLite with color/annotation columns",
		Example: "  notion-pp-cli format mirror 550e8400e29b41d4a716446655440000",
		// Reads the API page tree into a *local* SQLite mirror; no remote
		// mutation. Annotated read-only so MCP hosts don't gate it as a
		// destructive write. mcp:hidden so the typed `sync-page` MCP tool is
		// the single named surface for this capability.
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dbPath == "" {
				dbPath = defaultMirrorPath()
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"action": "mirror", "page": args[0], "db": dbPath, "dry_run": true})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openMirror(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer func() { _ = db.Close() }()

			page, err := c.Get(cmd.Context(), replacePathParam("/v1/pages/{page_id}", "page_id", args[0]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			pageID, title, url := pageMeta(page)
			if _, err := db.Exec(`INSERT OR REPLACE INTO pages (id,title,url,synced_at) VALUES (?,?,?,?)`,
				pageID, title, url, time.Now().UTC().Format(time.RFC3339)); err != nil {
				return configErr(err)
			}
			if _, err := db.Exec(`DELETE FROM blocks WHERE page_id=?`, pageID); err != nil {
				return configErr(err)
			}
			if _, err := db.Exec(`DELETE FROM rich_text WHERE page_id=?`, pageID); err != nil {
				return configErr(err)
			}

			count, err := mirrorWalk(cmd.Context(), c, db, pageID, pageID, 0)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{
				"action": "mirror", "page": pageID, "title": title,
				"blocks": count, "db": dbPath,
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default ~/.local/share/github.com/glbinv/notion-cli/mirror.db)")
	return cmd
}

// mirrorWalk recursively fetches children of parentID and stores them.
func mirrorWalk(ctx context.Context, c interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}, db *sql.DB, pageID, parentID string, depth int) (int, error) {
	count := 0
	cursor := ""
	pos := 0
	for {
		params := map[string]string{"page_size": "100"}
		if cursor != "" {
			params["start_cursor"] = cursor
		}
		raw, err := c.Get(ctx, replacePathParam("/v1/blocks/{block_id}/children", "block_id", parentID), params)
		if err != nil {
			return count, err
		}
		var resp struct {
			Results    []map[string]any `json:"results"`
			HasMore    bool             `json:"has_more"`
			NextCursor string           `json:"next_cursor"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return count, err
		}
		for _, b := range resp.Results {
			id, _ := b["id"].(string)
			btype, _ := b["type"].(string)
			hasChildren, _ := b["has_children"].(bool)
			lastEdited, _ := b["last_edited_time"].(string)
			typeObj, _ := b[btype].(map[string]any)
			color, _ := typeObj["color"].(string)
			rich, _ := typeObj["rich_text"].([]any)
			plain := plainText(rich)

			hc := 0
			if hasChildren {
				hc = 1
			}
			if _, err := db.Exec(`INSERT OR REPLACE INTO blocks (id,page_id,parent_id,type,position,depth,color,plain_text,has_children,last_edited_time) VALUES (?,?,?,?,?,?,?,?,?,?)`,
				id, pageID, parentID, btype, pos, depth, color, plain, hc, lastEdited); err != nil {
				return count, err
			}
			insertRichText(db, id, pageID, rich)
			count++
			pos++
			if hasChildren {
				n, err := mirrorWalk(ctx, c, db, pageID, id, depth+1)
				if err != nil {
					return count, err
				}
				count += n
			}
		}
		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return count, nil
}

func insertRichText(db *sql.DB, blockID, pageID string, rich []any) {
	for i, r := range rich {
		run, ok := r.(map[string]any)
		if !ok {
			continue
		}
		ann, _ := run["annotations"].(map[string]any)
		content, _ := run["plain_text"].(string)
		if content == "" {
			if t, ok := run["text"].(map[string]any); ok {
				content, _ = t["content"].(string)
			}
		}
		href, _ := run["href"].(string)
		_, _ = db.Exec(`INSERT INTO rich_text (block_id,page_id,position,content,color,bold,italic,strikethrough,underline,code,href) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			blockID, pageID, i, content, annStr(ann, "color", "default"),
			annBool(ann, "bold"), annBool(ann, "italic"), annBool(ann, "strikethrough"),
			annBool(ann, "underline"), annBool(ann, "code"), href)
	}
}

func annBool(ann map[string]any, key string) int {
	if ann != nil {
		if v, ok := ann[key].(bool); ok && v {
			return 1
		}
	}
	return 0
}

func annStr(ann map[string]any, key, def string) string {
	if ann != nil {
		if v, ok := ann[key].(string); ok && v != "" {
			return v
		}
	}
	return def
}

func plainText(rich []any) string {
	var b strings.Builder
	for _, r := range rich {
		run, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := run["plain_text"].(string); ok {
			b.WriteString(s)
		} else if t, ok := run["text"].(map[string]any); ok {
			if s, ok := t["content"].(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

func pageMeta(page json.RawMessage) (id, title, url string) {
	var p struct {
		ID         string                     `json:"id"`
		URL        string                     `json:"url"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(page, &p) != nil {
		return "", "(untitled)", ""
	}
	title = "(untitled)"
	for _, raw := range p.Properties {
		var prop struct {
			Type  string `json:"type"`
			Title []struct {
				PlainText string `json:"plain_text"`
			} `json:"title"`
		}
		if json.Unmarshal(raw, &prop) == nil && prop.Type == "title" {
			var sb strings.Builder
			for _, t := range prop.Title {
				sb.WriteString(t.PlainText)
			}
			if sb.Len() > 0 {
				title = sb.String()
			}
			break
		}
	}
	return p.ID, title, p.URL
}

// --- format sql ------------------------------------------------------------

func newFormatSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var schema bool
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a read-only SQL query over the local page mirror",
		Long: strings.Trim(`
Query the annotation-aware mirror built by 'format mirror'. Tables: pages,
blocks, rich_text. Views: colored_text, block_palette.`, "\n"),
		Example: "  notion-pp-cli format sql \"SELECT content,color FROM colored_text WHERE color LIKE '%red%'\"",
		// Opens the mirror DB mode=ro and runs SELECT-only queries. Annotated
		// read-only so MCP hosts surface it without a destructive-write prompt.
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultMirrorPath()
			}
			db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
			if err != nil {
				return configErr(fmt.Errorf("opening mirror %s: %w", dbPath, err))
			}
			defer func() { _ = db.Close() }()

			if schema {
				return runSQL(cmd, flags, db, `SELECT type,name FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY type,name`)
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			return runSQL(cmd, flags, db, strings.Join(args, " "))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default ~/.local/share/github.com/glbinv/notion-cli/mirror.db)")
	cmd.Flags().BoolVar(&schema, "schema", false, "List mirror tables and views")
	return cmd
}

func runSQL(cmd *cobra.Command, flags *rootFlags, db *sql.DB, query string) error {
	rows, err := db.QueryContext(cmd.Context(), query)
	if err != nil {
		return usageErr(fmt.Errorf("query failed: %w", err))
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return apiErr(err)
	}
	var out []map[string]any
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return apiErr(err)
		}
		row := map[string]any{}
		for i, c := range cols {
			v := cells[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[c] = v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return apiErr(err)
	}
	return flags.printJSON(cmd, out)
}

func openMirror(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(mirrorSchema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
