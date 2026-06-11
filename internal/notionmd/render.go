// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// render.go is hand-authored (NOT generated). It is the inverse of the
// ParseMarkdown writer: it serializes a fetched Notion block tree back into
// the extended-markdown dialect, round-tripping the block-level color and
// per-run annotations that the official Notion MCP drops on read. It powers
// the `format read-page` command.

package notionmd

import (
	"fmt"
	"strings"
)

// RenderedBlock is a Notion block object (as returned by the API) augmented
// with a Children slice the caller populates by recursively fetching
// /v1/blocks/{id}/children. Keeping the raw object as a map mirrors the
// writer side, which also treats blocks as untyped maps.
type RenderedBlock struct {
	Object   map[string]any
	Children []RenderedBlock
}

// RenderBlocks serializes a block tree into extended markdown. Nested blocks
// (callout/toggle bodies, list children) are indented two spaces per level so
// the output re-parses through ParseMarkdown into the same structure.
func RenderBlocks(blocks []RenderedBlock) string {
	var b strings.Builder
	renderBlockList(&b, blocks, 0)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func renderBlockList(b *strings.Builder, blocks []RenderedBlock, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, blk := range blocks {
		obj := blk.Object
		btype, _ := obj["type"].(string)
		payload, _ := obj[btype].(map[string]any)
		if payload == nil {
			payload = map[string]any{}
		}
		rich, _ := payload["rich_text"].([]any)
		text := renderInline(rich)
		color, _ := payload["color"].(string)

		switch btype {
		case "heading_1":
			fmt.Fprintf(b, "%s# %s%s\n", indent, text, blockAttr(color))
		case "heading_2":
			fmt.Fprintf(b, "%s## %s%s\n", indent, text, blockAttr(color))
		case "heading_3":
			fmt.Fprintf(b, "%s### %s%s\n", indent, text, blockAttr(color))
		case "bulleted_list_item":
			fmt.Fprintf(b, "%s- %s%s\n", indent, text, blockAttr(color))
		case "numbered_list_item":
			fmt.Fprintf(b, "%s1. %s%s\n", indent, text, blockAttr(color))
		case "to_do":
			box := "[ ]"
			if checked, _ := payload["checked"].(bool); checked {
				box = "[x]"
			}
			fmt.Fprintf(b, "%s- %s %s%s\n", indent, box, text, blockAttr(color))
		case "quote":
			fmt.Fprintf(b, "%s> %s%s\n", indent, text, blockAttr(color))
		case "callout":
			fmt.Fprintf(b, "%s> [!callout]%s %s\n", indent, calloutAttrs(payload, color), text)
		case "toggle":
			fmt.Fprintf(b, "%s> [!toggle] %s%s\n", indent, text, blockAttr(color))
		case "code":
			lang, _ := payload["language"].(string)
			fmt.Fprintf(b, "%s```%s\n%s%s\n%s```\n", indent, lang, indent, plainConcat(rich), indent)
		case "divider":
			fmt.Fprintf(b, "%s---\n", indent)
		case "":
			// Unknown/unsupported shape — skip silently.
		default:
			// Render any other text-bearing block as a paragraph so its
			// content (and color) survives the round-trip rather than being
			// dropped. Pure structural blocks with no rich_text emit nothing.
			if text != "" {
				fmt.Fprintf(b, "%s%s%s\n", indent, text, blockAttr(color))
			}
		}

		if len(blk.Children) > 0 {
			renderBlockList(b, blk.Children, depth+1)
		}
	}
}

// renderInline serializes a slice of rich_text runs back into the inline
// extended-markdown dialect. Annotation order is chosen so the result
// re-parses cleanly: code spans take precedence (their contents are literal),
// then bold/italic/strike/underline wrappers, then a {color:..} group, then a
// link wrapper on the outside.
func renderInline(rich []any) string {
	var b strings.Builder
	for _, r := range rich {
		run, ok := r.(map[string]any)
		if !ok {
			continue
		}
		s := runText(run)
		if s == "" {
			continue
		}
		ann, _ := run["annotations"].(map[string]any)
		if mBool(ann, "code") {
			s = "`" + s + "`"
		} else {
			if mBool(ann, "bold") {
				s = "**" + s + "**"
			}
			if mBool(ann, "italic") {
				s = "*" + s + "*"
			}
			if mBool(ann, "strikethrough") {
				s = "~~" + s + "~~"
			}
			if mBool(ann, "underline") {
				s = "<u>" + s + "</u>"
			}
			if c := mStr(ann, "color"); c != "" && c != "default" {
				s = "{" + colorToken(c) + ":" + s + "}"
			}
		}
		if href := runHref(run); href != "" {
			s = "[" + s + "](" + href + ")"
		}
		b.WriteString(s)
	}
	return b.String()
}

// blockAttr renders a trailing block-level attribute group for a non-default
// block color, matching extractAttrs on the writer side ({color=..} for a
// foreground color, {bg=..} for a background color).
func blockAttr(color string) string {
	if color == "" || color == "default" {
		return ""
	}
	if base, ok := strings.CutSuffix(color, "_background"); ok {
		return " {bg=" + base + "}"
	}
	return " {color=" + color + "}"
}

// calloutAttrs renders the leading attribute group for a callout
// (`{bg=.. icon=..}`) from its color and icon, matching extractLeadingAttrs.
func calloutAttrs(payload map[string]any, color string) string {
	var parts []string
	if color != "" && color != "default" {
		if base, ok := strings.CutSuffix(color, "_background"); ok {
			parts = append(parts, "bg="+base)
		} else {
			parts = append(parts, "color="+color)
		}
	}
	if icon, ok := payload["icon"].(map[string]any); ok {
		if emoji, ok := icon["emoji"].(string); ok && emoji != "" {
			parts = append(parts, "icon="+emoji)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " {" + strings.Join(parts, " ") + "}"
}

// colorToken converts a Notion color to its writer token: foreground colors
// pass through ("red"), background colors use the "_bg" shorthand
// ("red_background" -> "red_bg") that NormalizeColor accepts.
func colorToken(color string) string {
	if base, ok := strings.CutSuffix(color, "_background"); ok {
		return base + "_bg"
	}
	return color
}

// runText returns a run's text content, preferring plain_text (present on API
// responses) and falling back to text.content.
func runText(run map[string]any) string {
	if s, ok := run["plain_text"].(string); ok && s != "" {
		return s
	}
	if t, ok := run["text"].(map[string]any); ok {
		if s, ok := t["content"].(string); ok {
			return s
		}
	}
	return ""
}

// runHref returns a run's link URL from either the top-level href or the
// nested text.link.url, or "" when the run is not a link.
func runHref(run map[string]any) string {
	if h, ok := run["href"].(string); ok && h != "" {
		return h
	}
	if t, ok := run["text"].(map[string]any); ok {
		if l, ok := t["link"].(map[string]any); ok {
			if u, ok := l["url"].(string); ok {
				return u
			}
		}
	}
	return ""
}

func mBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

func mStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// plainConcat joins the literal text of every run, ignoring annotations. Used
// for code blocks, whose contents are verbatim.
func plainConcat(rich []any) string {
	var b strings.Builder
	for _, r := range rich {
		run, ok := r.(map[string]any)
		if !ok {
			continue
		}
		b.WriteString(runText(run))
	}
	return b.String()
}
