package notionmd

import (
	"fmt"
	"regexp"
	"strings"
)

// Block is a Notion block object. The append/update endpoints accept arbitrary
// JSON, so a map keeps the many block-type payloads simple.
type Block = map[string]any

// EmojiIcon builds a callout icon object.
func emojiIcon(emoji string) map[string]any {
	return map[string]any{"type": "emoji", "emoji": emoji}
}

// --- code-fence language mapping -------------------------------------------

var codeLanguages = map[string]bool{
	"abap": true, "arduino": true, "bash": true, "basic": true, "c": true,
	"clojure": true, "coffeescript": true, "c++": true, "c#": true, "css": true,
	"dart": true, "diff": true, "docker": true, "elixir": true, "elm": true,
	"erlang": true, "flow": true, "fortran": true, "f#": true, "gherkin": true,
	"glsl": true, "go": true, "graphql": true, "groovy": true, "haskell": true,
	"html": true, "java": true, "javascript": true, "json": true, "julia": true,
	"kotlin": true, "latex": true, "less": true, "lisp": true, "livescript": true,
	"lua": true, "makefile": true, "markdown": true, "markup": true, "matlab": true,
	"mermaid": true, "nix": true, "objective-c": true, "ocaml": true, "pascal": true,
	"perl": true, "php": true, "plain text": true, "powershell": true, "prolog": true,
	"protobuf": true, "python": true, "r": true, "reason": true, "ruby": true,
	"rust": true, "sass": true, "scala": true, "scheme": true, "scss": true,
	"shell": true, "sql": true, "swift": true, "typescript": true, "vb.net": true,
	"verilog": true, "vhdl": true, "visual basic": true, "webassembly": true,
	"xml": true, "yaml": true,
}

var langAliases = map[string]string{
	"js": "javascript", "ts": "typescript", "py": "python", "sh": "shell",
	"ps1": "powershell", "yml": "yaml", "cpp": "c++", "cs": "c#",
	"rb": "ruby", "rs": "rust", "kt": "kotlin", "md": "markdown",
}

func mapLanguage(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if l == "" {
		return "plain text"
	}
	if a, ok := langAliases[l]; ok {
		l = a
	}
	if codeLanguages[l] {
		return l
	}
	return "plain text"
}

// --- block builders --------------------------------------------------------

func withChildren(payload map[string]any, children []Block) map[string]any {
	if len(children) > 0 {
		payload["children"] = children
	}
	return payload
}

func paragraph(rich []RichText, color string) Block {
	return Block{"object": "block", "type": "paragraph", "paragraph": map[string]any{"rich_text": rich, "color": color}}
}

func heading(level int, rich []RichText, color string) Block {
	key := fmt.Sprintf("heading_%d", level)
	return Block{"object": "block", "type": key, key: map[string]any{"rich_text": rich, "color": color}}
}

func bulletedListItem(rich []RichText, color string, children []Block) Block {
	return Block{"object": "block", "type": "bulleted_list_item", "bulleted_list_item": withChildren(map[string]any{"rich_text": rich, "color": color}, children)}
}

func numberedListItem(rich []RichText, color string, children []Block) Block {
	return Block{"object": "block", "type": "numbered_list_item", "numbered_list_item": withChildren(map[string]any{"rich_text": rich, "color": color}, children)}
}

func toDo(rich []RichText, checked bool, color string, children []Block) Block {
	return Block{"object": "block", "type": "to_do", "to_do": withChildren(map[string]any{"rich_text": rich, "checked": checked, "color": color}, children)}
}

func quote(rich []RichText, color string, children []Block) Block {
	return Block{"object": "block", "type": "quote", "quote": withChildren(map[string]any{"rich_text": rich, "color": color}, children)}
}

func callout(rich []RichText, color string, icon map[string]any, children []Block) Block {
	if color == "" {
		color = "gray_background"
	}
	if icon == nil {
		icon = emojiIcon("💡")
	}
	return Block{"object": "block", "type": "callout", "callout": withChildren(map[string]any{"rich_text": rich, "color": color, "icon": icon}, children)}
}

func toggle(rich []RichText, color string, children []Block) Block {
	return Block{"object": "block", "type": "toggle", "toggle": withChildren(map[string]any{"rich_text": rich, "color": color}, children)}
}

func codeBlock(text, language string) Block {
	rich := []RichText{{Type: "text", Text: TextContent{Content: text}, Annotations: Annotations{Color: "default"}}}
	return Block{"object": "block", "type": "code", "code": map[string]any{"rich_text": rich, "language": mapLanguage(language)}}
}

func divider() Block {
	return Block{"object": "block", "type": "divider", "divider": map[string]any{}}
}

// --- block-level attributes: trailing {key=value ...} ----------------------

type blockAttrs struct {
	color string
	bg    string
	icon  map[string]any
}

var iconShortcodes = map[string]string{
	"rocket": "🚀", "warning": "⚠️", "warn": "⚠️", "danger": "🛑", "stop": "🛑",
	"info": "ℹ️", "note": "📝", "pencil": "📝", "check": "✅", "done": "✅",
	"fire": "🔥", "bulb": "💡", "idea": "💡", "tip": "💡", "star": "⭐",
	"bug": "🐛", "question": "❓", "lock": "🔒", "calendar": "📅", "chart": "📊",
}

func resolveIcon(v string) map[string]any {
	key := strings.ToLower(strings.TrimSpace(v))
	if e, ok := iconShortcodes[key]; ok {
		return emojiIcon(e)
	}
	return emojiIcon(strings.TrimSpace(v))
}

var (
	attrTrailerRe = regexp.MustCompile(`\s*\{([^{}]*=[^{}]*)\}\s*$`)
	attrLeaderRe  = regexp.MustCompile(`^\s*\{([^{}]*=[^{}]*)\}\s*`)
)

func parseAttrTokens(inner string, attrs *blockAttrs) {
	for _, tok := range strings.Fields(inner) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(k) {
		case "color":
			if c, ok := NormalizeColor(v); ok {
				attrs.color = c
			}
		case "bg", "background":
			val := v
			if !strings.HasSuffix(val, "_background") {
				val += "_background"
			}
			if c, ok := NormalizeColor(val); ok {
				attrs.bg = c
			}
		case "icon":
			attrs.icon = resolveIcon(v)
		}
	}
}

// extractAttrs strips a trailing {key=value ...} group from a block line
// (used by headings, quotes, paragraphs, list items).
func extractAttrs(line string) (string, blockAttrs) {
	var attrs blockAttrs
	loc := attrTrailerRe.FindStringSubmatchIndex(line)
	if loc == nil {
		return line, attrs
	}
	parseAttrTokens(line[loc[2]:loc[3]], &attrs)
	return strings.TrimRight(line[:loc[0]], " "), attrs
}

// extractLeadingAttrs strips a leading {key=value ...} group — the documented
// position for admonition attributes (`[!callout] {bg=blue icon=rocket} Body`).
func extractLeadingAttrs(line string) (string, blockAttrs) {
	var attrs blockAttrs
	loc := attrLeaderRe.FindStringSubmatchIndex(line)
	if loc == nil {
		return line, attrs
	}
	parseAttrTokens(line[loc[2]:loc[3]], &attrs)
	return line[loc[1]:], attrs
}

func blockColor(a blockAttrs, fallback string) string {
	if a.bg != "" {
		return a.bg
	}
	if a.color != "" {
		return a.color
	}
	return fallback
}

// --- block parser ----------------------------------------------------------

var (
	reFence      = regexp.MustCompile("^```(.*)$")
	reDivider    = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})\s*$`)
	reHeading    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reQuote      = regexp.MustCompile(`^>\s?(.*)$`)
	reTodo       = regexp.MustCompile(`^([-+*])\s+\[([ xX])\]\s+(.*)$`)
	reBullet     = regexp.MustCompile(`^([-+*])\s+(.*)$`)
	reNumbered   = regexp.MustCompile(`^(\d+)[.)]\s+(.*)$`)
	reAdmonition = regexp.MustCompile(`^\[!(\w+)\]\s*(.*)$`)
)

func indentOf(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func isListLine(c string) bool {
	return reTodo.MatchString(c) || reBullet.MatchString(c) || reNumbered.MatchString(c)
}

func isBlockStart(c string) bool {
	return reFence.MatchString(c) || reDivider.MatchString(c) || reHeading.MatchString(c) ||
		reQuote.MatchString(c) || isListLine(c)
}

// ParseMarkdown converts an extended-markdown document into Notion blocks.
func ParseMarkdown(md string) []Block {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.ReplaceAll(md, "\r", "\n")
	md = strings.ReplaceAll(md, "\t", "  ")
	return parseLines(strings.Split(md, "\n"))
}

func dedent(lines []string) []string {
	min := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := indentOf(l); min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= min {
			out[i] = l[min:]
		} else {
			out[i] = l
		}
	}
	return out
}

func parseLines(lines []string) []Block {
	out := []Block{}
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if strings.TrimSpace(raw) == "" {
			i++
			continue
		}
		indent := indentOf(raw)
		content := raw[indent:]

		if m := reFence.FindStringSubmatch(content); m != nil { // code fence
			lang := strings.TrimSpace(m[1])
			var body []string
			i++
			for i < len(lines) {
				c := lines[i][indentOf(lines[i]):]
				if strings.HasPrefix(c, "```") {
					i++
					break
				}
				strip := indent
				if li := indentOf(lines[i]); li < strip {
					strip = li
				}
				body = append(body, lines[i][strip:])
				i++
			}
			out = append(out, codeBlock(strings.Join(body, "\n"), lang))
			continue
		}
		if reDivider.MatchString(content) {
			out = append(out, divider())
			i++
			continue
		}
		if m := reHeading.FindStringSubmatch(content); m != nil {
			level := len(m[1])
			if level > 3 {
				level = 3
			}
			text, attrs := extractAttrs(m[2])
			out = append(out, heading(level, ParseInline(text), blockColor(attrs, "default")))
			i++
			continue
		}
		if reQuote.MatchString(content) { // blockquote / admonition
			var group []string
			for i < len(lines) {
				c := lines[i][indentOf(lines[i]):]
				m := reQuote.FindStringSubmatch(c)
				if m == nil {
					break
				}
				group = append(group, m[1])
				i++
			}
			out = append(out, parseBlockquote(group))
			continue
		}
		if isListLine(content) {
			blocks, next := parseList(lines, i, indent)
			out = append(out, blocks...)
			i = next
			continue
		}

		// paragraph: gather consecutive non-blank, non-block-start lines
		var para []string
		for i < len(lines) {
			r := lines[i]
			if strings.TrimSpace(r) == "" {
				break
			}
			c := r[indentOf(r):]
			if isBlockStart(c) {
				break
			}
			para = append(para, c)
			i++
		}
		text, attrs := extractAttrs(strings.Join(para, "\n"))
		out = append(out, paragraph(ParseInline(text), blockColor(attrs, "default")))
	}
	return out
}

func parseBlockquote(group []string) Block {
	first := ""
	if len(group) > 0 {
		first = group[0]
	}
	if m := reAdmonition.FindStringSubmatch(first); m != nil {
		kind := strings.ToLower(m[1])
		// Admonition attributes are documented as leading ({bg=..} right after
		// [!type]); also accept a trailing group and merge (leading wins).
		head, attrs := extractLeadingAttrs(m[2])
		head, trailing := extractAttrs(head)
		if attrs.color == "" {
			attrs.color = trailing.color
		}
		if attrs.bg == "" {
			attrs.bg = trailing.bg
		}
		if attrs.icon == nil {
			attrs.icon = trailing.icon
		}
		body := group[1:]
		var children []Block
		hasBody := false
		for _, l := range body {
			if strings.TrimSpace(l) != "" {
				hasBody = true
				break
			}
		}
		if hasBody {
			children = parseLines(body)
		}
		if kind == "toggle" {
			return toggle(ParseInline(head), blockColor(attrs, "default"), children)
		}
		color := attrs.bg
		if color == "" {
			color = attrs.color
		}
		if color == "" {
			color = "gray_background"
		}
		return callout(ParseInline(head), color, attrs.icon, children)
	}
	text, attrs := extractAttrs(strings.Join(group, "\n"))
	return quote(ParseInline(text), blockColor(attrs, "default"), nil)
}

type parsedItem struct {
	kind    string // bulleted | numbered | todo
	rich    []RichText
	color   string
	checked bool
}

func parseListItem(content string) parsedItem {
	if m := reTodo.FindStringSubmatch(content); m != nil {
		text, attrs := extractAttrs(m[3])
		return parsedItem{kind: "todo", rich: ParseInline(text), color: blockColor(attrs, "default"), checked: strings.EqualFold(m[2], "x")}
	}
	if m := reNumbered.FindStringSubmatch(content); m != nil {
		text, attrs := extractAttrs(m[2])
		return parsedItem{kind: "numbered", rich: ParseInline(text), color: blockColor(attrs, "default")}
	}
	if m := reBullet.FindStringSubmatch(content); m != nil {
		text, attrs := extractAttrs(m[2])
		return parsedItem{kind: "bulleted", rich: ParseInline(text), color: blockColor(attrs, "default")}
	}
	return parsedItem{kind: "bulleted", rich: ParseInline(content), color: "default"}
}

func buildListItem(it parsedItem, children []Block) Block {
	switch it.kind {
	case "todo":
		return toDo(it.rich, it.checked, it.color, children)
	case "numbered":
		return numberedListItem(it.rich, it.color, children)
	default:
		return bulletedListItem(it.rich, it.color, children)
	}
}

func parseList(lines []string, start, baseIndent int) ([]Block, int) {
	out := []Block{}
	i := start
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" { // allow a single blank between same-level items
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) && indentOf(lines[j]) == baseIndent && isListLine(lines[j][baseIndent:]) {
				i = j
				continue
			}
			break
		}
		indent := indentOf(lines[i])
		if indent != baseIndent {
			break
		}
		content := lines[i][indent:]
		if !isListLine(content) {
			break
		}
		item := parseListItem(content)
		i++

		var childLines []string
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "" {
				j := i + 1
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				if j < len(lines) && indentOf(lines[j]) > baseIndent {
					childLines = append(childLines, "")
					i++
					continue
				}
				break
			}
			if indentOf(lines[i]) > baseIndent {
				childLines = append(childLines, lines[i])
				i++
			} else {
				break
			}
		}
		var children []Block
		if len(childLines) > 0 {
			children = parseLines(dedent(childLines))
		}
		out = append(out, buildListItem(item, children))
	}
	return out, i
}
