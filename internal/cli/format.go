// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// format.go is hand-authored (NOT generated). It implements notion-pp-cli's
// formatting-first writer: an extended-markdown dialect that preserves the
// block-level color and annotation fidelity the official Notion MCP drops.
// See internal/notionmd for the parser.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"notion-pp-cli/internal/notionmd"

	"github.com/spf13/cobra"
)

func newFormatCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Formatting-first writer: extended markdown with full Notion color & annotation control",
		Long: strings.Trim(`
Write and re-style Notion blocks with color and annotation fidelity the
official MCP loses. Extended-markdown dialect:

  **bold**  *italic*  `+"`code`"+`  ~~strike~~  <u>underline</u>  ==highlight==
  {red:text}            text color (any of: gray brown orange yellow green
                        blue purple pink red)
  {red_bg:text}         background color (also {red_background:text})
  [label](https://url)  link (label may itself be formatted)

Block-level, via a trailing {..} attribute group:
  # Heading {color=blue}
  > Quote {bg=gray}
  > [!callout] {bg=blue icon=rocket} Body text with {green:inline} color
  > [!toggle] Summary {color=purple}
`, "\n"),
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFormatPreviewCmd(flags))
	cmd.AddCommand(newFormatAppendCmd(flags))
	cmd.AddCommand(newFormatWritePageCmd(flags))
	cmd.AddCommand(newFormatUpdateCmd(flags))
	cmd.AddCommand(newFormatMirrorCmd(flags))
	cmd.AddCommand(newFormatSQLCmd(flags))
	cmd.AddCommand(newFormatReadPageCmd(flags))
	return cmd
}

// resolveMarkdown returns markdown text from --md or --file. The file read
// happens only when not in dry-run (verify probes commands with --dry-run and
// fixture paths that do not exist).
func resolveMarkdown(md, file string, dryRun bool) (string, error) {
	if md != "" {
		return md, nil
	}
	if file != "" && !dryRun {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", file, err)
		}
		return string(b), nil
	}
	return "", nil
}

// --- format preview --------------------------------------------------------

func newFormatPreviewCmd(flags *rootFlags) *cobra.Command {
	var md, file string
	cmd := &cobra.Command{
		Use:     "preview",
		Short:   "Parse extended markdown to Notion block JSON without sending it",
		Example: "  notion-pp-cli format preview --md '{red:urgent} and ==flagged== and **bold**'",
		// Pure local parse — never contacts the API. Annotated read-only so MCP
		// hosts surface it without a per-call destructive-write prompt.
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := resolveMarkdown(md, file, false)
			if err != nil {
				return err
			}
			if text == "" {
				return cmd.Help()
			}
			blocks := notionmd.ParseMarkdown(text)
			return flags.printJSON(cmd, blocks)
		},
	}
	cmd.Flags().StringVarP(&md, "md", "m", "", "Extended-markdown source text")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Read extended markdown from a file")
	return cmd
}

// --- format append ---------------------------------------------------------

func newFormatAppendCmd(flags *rootFlags) *cobra.Command {
	var md, file, after string
	cmd := &cobra.Command{
		Use:     "append <parent_id>",
		Short:   "Append color-formatted blocks (from extended markdown) to a page or block",
		Example: "  notion-pp-cli format append 550e8400e29b41d4a716446655440000 --md '> [!callout] {bg=blue icon=rocket} Shipping today'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			text, err := resolveMarkdown(md, file, dryRunOK(flags))
			if err != nil {
				return err
			}
			blocks := notionmd.ParseMarkdown(text)
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"action": "append", "parent": args[0], "dry_run": true,
					"block_count": len(blocks), "blocks": blocks,
				})
			}
			if len(blocks) == 0 {
				return usageErr(errors.New("nothing to append: provide --md <text> or --file <path>"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/v1/blocks/{block_id}/children", "block_id", args[0])
			body := map[string]any{"children": blocks}
			if after != "" {
				body["after"] = after
			}
			data, status, err := c.PatchWithParams(cmd.Context(), path, map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitWriteResult(cmd, flags, "append", args[0], status, len(blocks), data)
		},
	}
	cmd.Flags().StringVarP(&md, "md", "m", "", "Extended-markdown source text")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Read extended markdown from a file")
	cmd.Flags().StringVar(&after, "after", "", "Insert after this existing child block ID")
	return cmd
}

// --- format write-page -----------------------------------------------------

func newFormatWritePageCmd(flags *rootFlags) *cobra.Command {
	var file, md, mode string
	cmd := &cobra.Command{
		Use:     "write-page <page_id>",
		Short:   "Render an extended-markdown file to a page (append or replace)",
		Example: "  notion-pp-cli format write-page 550e8400e29b41d4a716446655440000 --from-markdown notes.md --mode replace",
		// mcp:hidden — surfaced to MCP hosts as the typed `format-write-page`
		// tool instead of an auto-generated cobra-tree mirror tool.
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if mode != "append" && mode != "replace" {
				return usageErr(fmt.Errorf("invalid --mode %q: must be append or replace", mode))
			}
			src := file
			text, err := resolveMarkdown(md, src, dryRunOK(flags))
			if err != nil {
				return err
			}
			blocks := notionmd.ParseMarkdown(text)
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"action": "write-page", "page": args[0], "mode": mode,
					"dry_run": true, "block_count": len(blocks), "blocks": blocks,
				})
			}
			if len(blocks) == 0 {
				return usageErr(errors.New("nothing to write: provide --from-markdown <path> or --md <text>"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if mode == "replace" {
				ids, err := collectChildIDs(cmd.Context(), c, args[0])
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, id := range ids {
					if _, _, derr := c.Delete(cmd.Context(), replacePathParam("/v1/blocks/{block_id}", "block_id", id)); derr != nil {
						return classifyAPIError(derr, flags)
					}
				}
				fmt.Fprintf(os.Stderr, "archived %d existing block(s)\n", len(ids))
			}
			path := replacePathParam("/v1/blocks/{block_id}/children", "block_id", args[0])
			data, status, err := c.PatchWithParams(cmd.Context(), path, map[string]string{}, map[string]any{"children": blocks})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitWriteResult(cmd, flags, "write-page", args[0], status, len(blocks), data)
		},
	}
	cmd.Flags().StringVar(&file, "from-markdown", "", "Extended-markdown file to render")
	cmd.Flags().StringVarP(&md, "md", "m", "", "Inline extended-markdown source (alternative to --from-markdown)")
	cmd.Flags().StringVar(&mode, "mode", "append", "append | replace (replace archives existing blocks first)")
	return cmd
}

// --- format update (in-place reformatting) ---------------------------------

func newFormatUpdateCmd(flags *rootFlags) *cobra.Command {
	var color, recolor, setAnns, unsetAnns, icon, md string
	var check, uncheck bool
	cmd := &cobra.Command{
		Use:   "update <block_id>",
		Short: "Re-style an existing block's formatting in place (without rewriting its text)",
		Long: strings.Trim(`
Change a block's formatting without retyping its content. --recolor and
--set/--unset rewrite the annotations on every existing rich-text run while
preserving the text. --color sets the block-level color; --icon sets a
callout icon; --check/--uncheck toggle a to_do.`, "\n"),
		Example: "  notion-pp-cli format update 550e8400e29b41d4a716446655440000 --recolor blue --set bold,code",
		// mcp:hidden — surfaced to MCP hosts as the typed `format-update-block` tool.
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"action": "update", "block": args[0], "dry_run": true})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), replacePathParam("/v1/blocks/{block_id}", "block_id", args[0]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var block map[string]any
			if err := json.Unmarshal(raw, &block); err != nil {
				return apiErr(fmt.Errorf("parsing block: %w", err))
			}
			btype, _ := block["type"].(string)
			payload, err := buildUpdatePayload(btype, block, updateOpts{
				color: color, recolor: recolor, setAnns: setAnns, unsetAnns: unsetAnns,
				icon: icon, md: md, check: check, uncheck: uncheck,
			})
			if err != nil {
				return usageErr(err)
			}
			data, status, err := c.PatchWithParams(cmd.Context(), replacePathParam("/v1/blocks/{block_id}", "block_id", args[0]), map[string]string{}, payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitWriteResult(cmd, flags, "update", args[0], status, 1, data)
		},
	}
	cmd.Flags().StringVar(&color, "color", "", "Set the block-level color (e.g. blue, red_bg)")
	cmd.Flags().StringVar(&recolor, "recolor", "", "Set annotations.color on every existing run (keeps the text)")
	cmd.Flags().StringVar(&setAnns, "set", "", "Enable annotations on every run: bold,italic,underline,strike,code")
	cmd.Flags().StringVar(&unsetAnns, "unset", "", "Disable annotations on every run")
	cmd.Flags().StringVar(&icon, "icon", "", "Set a callout icon (emoji or shortcode, e.g. rocket)")
	cmd.Flags().StringVarP(&md, "md", "m", "", "Replace the block's rich text from extended markdown")
	cmd.Flags().BoolVar(&check, "check", false, "Mark a to_do block checked")
	cmd.Flags().BoolVar(&uncheck, "uncheck", false, "Mark a to_do block unchecked")
	return cmd
}

type updateOpts struct {
	color, recolor, setAnns, unsetAnns, icon, md string
	check, uncheck                               bool
}

var annotationKeys = map[string]string{
	"bold": "bold", "italic": "italic", "underline": "underline",
	"strike": "strikethrough", "strikethrough": "strikethrough", "code": "code",
}

func buildUpdatePayload(btype string, block map[string]any, o updateOpts) (map[string]any, error) {
	typeObj, _ := block[btype].(map[string]any)
	if typeObj == nil {
		typeObj = map[string]any{}
	}
	out := map[string]any{}
	touchedRich := false

	// Start from existing rich text (as []any of map[string]any)
	rich, _ := typeObj["rich_text"].([]any)

	if o.md != "" {
		parsed := notionmd.ParseInline(o.md)
		b, _ := json.Marshal(parsed)
		var asAny []any
		_ = json.Unmarshal(b, &asAny)
		rich = asAny
		touchedRich = true
	}
	if o.recolor != "" {
		col, ok := notionmd.NormalizeColor(o.recolor)
		if !ok {
			return nil, fmt.Errorf("unknown color: %s", o.recolor)
		}
		applyToRuns(rich, func(a map[string]any) { a["color"] = col })
		touchedRich = true
	}
	for _, name := range splitCSV(o.setAnns) {
		key, ok := annotationKeys[name]
		if !ok {
			return nil, fmt.Errorf("unknown annotation %q (use: bold,italic,underline,strike,code)", name)
		}
		applyToRuns(rich, func(a map[string]any) { a[key] = true })
		touchedRich = true
	}
	for _, name := range splitCSV(o.unsetAnns) {
		key, ok := annotationKeys[name]
		if !ok {
			return nil, fmt.Errorf("unknown annotation %q (use: bold,italic,underline,strike,code)", name)
		}
		applyToRuns(rich, func(a map[string]any) { a[key] = false })
		touchedRich = true
	}

	if touchedRich {
		out["rich_text"] = rich
	}
	if o.color != "" {
		col, ok := notionmd.NormalizeColor(o.color)
		if !ok {
			return nil, fmt.Errorf("unknown color: %s", o.color)
		}
		out["color"] = col
	}
	if o.icon != "" {
		if btype != "callout" {
			return nil, errors.New("--icon only applies to callout blocks")
		}
		out["icon"] = map[string]any{"type": "emoji", "emoji": iconEmoji(o.icon)}
	}
	if o.check || o.uncheck {
		if btype != "to_do" {
			return nil, errors.New("--check/--uncheck only apply to to_do blocks")
		}
		out["checked"] = o.check && !o.uncheck
	}
	if len(out) == 0 {
		return nil, errors.New("nothing to update: use --color/--recolor/--set/--unset/--icon/--check/--md")
	}
	return map[string]any{btype: out}, nil
}

// applyToRuns mutates the annotations object of each rich-text run.
func applyToRuns(rich []any, fn func(map[string]any)) {
	for _, r := range rich {
		run, ok := r.(map[string]any)
		if !ok {
			continue
		}
		ann, ok := run["annotations"].(map[string]any)
		if !ok {
			ann = map[string]any{}
			run["annotations"] = ann
		}
		fn(ann)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// iconEmoji resolves a shortcode (e.g. "rocket") to an emoji, else passes the
// value through as a literal emoji.
func iconEmoji(v string) string {
	shortcodes := map[string]string{
		"rocket": "🚀", "warning": "⚠️", "info": "ℹ️", "note": "📝", "check": "✅",
		"fire": "🔥", "bulb": "💡", "idea": "💡", "star": "⭐", "bug": "🐛",
		"lock": "🔒", "calendar": "📅", "chart": "📊", "question": "❓",
	}
	if e, ok := shortcodes[strings.ToLower(strings.TrimSpace(v))]; ok {
		return e
	}
	return strings.TrimSpace(v)
}

// collectChildIDs returns every direct child block ID of a page/block,
// following pagination.
func collectChildIDs(ctx context.Context, c interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}, parentID string) ([]string, error) {
	var ids []string
	cursor := ""
	for {
		params := map[string]string{"page_size": "100"}
		if cursor != "" {
			params["start_cursor"] = cursor
		}
		raw, err := c.Get(ctx, replacePathParam("/v1/blocks/{block_id}/children", "block_id", parentID), params)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		for _, r := range resp.Results {
			if r.ID != "" {
				ids = append(ids, r.ID)
			}
		}
		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return ids, nil
}

// --- format read-page (inverse of write-page) ------------------------------

func newFormatReadPageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "read-page <page_id>",
		Short:   "Render a page's block tree back to extended markdown (inverse of write-page), preserving color & annotations",
		Example: "  notion-pp-cli format read-page 550e8400e29b41d4a716446655440000",
		// Read-only: fetches the block tree and renders it locally via
		// internal/notionmd; no remote mutation. mcp:hidden so the typed
		// `format-read-page` tool is the single named MCP surface.
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"action": "read-page", "page": args[0], "dry_run": true})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tree, err := readPageTree(cmd.Context(), c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			md := notionmd.RenderBlocks(tree)
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"action": "read-page", "page": args[0],
					"block_count": len(tree), "markdown": md,
				})
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), md)
			return nil
		},
	}
	return cmd
}

// readPageTree recursively fetches the block tree under parentID and returns
// it as notionmd.RenderedBlock nodes (children attached) ready for rendering.
func readPageTree(ctx context.Context, c interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}, parentID string) ([]notionmd.RenderedBlock, error) {
	var out []notionmd.RenderedBlock
	cursor := ""
	for {
		params := map[string]string{"page_size": "100"}
		if cursor != "" {
			params["start_cursor"] = cursor
		}
		raw, err := c.Get(ctx, replacePathParam("/v1/blocks/{block_id}/children", "block_id", parentID), params)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Results    []map[string]any `json:"results"`
			HasMore    bool             `json:"has_more"`
			NextCursor string           `json:"next_cursor"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		for _, blk := range resp.Results {
			node := notionmd.RenderedBlock{Object: blk}
			if hasChildren, _ := blk["has_children"].(bool); hasChildren {
				if id, _ := blk["id"].(string); id != "" {
					kids, err := readPageTree(ctx, c, id)
					if err != nil {
						return nil, err
					}
					node.Children = kids
				}
			}
			out = append(out, node)
		}
		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return out, nil
}

// emitWriteResult prints a concise envelope for a successful write.
func emitWriteResult(cmd *cobra.Command, flags *rootFlags, action, target string, status, blockCount int, data json.RawMessage) error {
	env := map[string]any{
		"action":      action,
		"target":      target,
		"status":      status,
		"block_count": blockCount,
		"success":     status >= 200 && status < 300,
	}
	if len(data) > 0 {
		var parsed any
		if json.Unmarshal(data, &parsed) == nil {
			env["response"] = parsed
		}
	}
	return flags.printJSON(cmd, env)
}
