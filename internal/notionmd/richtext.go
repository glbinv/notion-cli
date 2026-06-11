package notionmd

import (
	"regexp"
	"strings"
)

const maxRichText = 2000 // Notion's per-rich-text content limit

// Annotations mirrors Notion's rich_text annotations object.
type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// Link is a rich_text text.link object.
type Link struct {
	URL string `json:"url"`
}

// TextContent is the rich_text text object.
type TextContent struct {
	Content string `json:"content"`
	Link    *Link  `json:"link"` // nil marshals to null, which Notion requires
}

// RichText is a single Notion rich_text run.
type RichText struct {
	Type        string      `json:"type"` // always "text"
	Text        TextContent `json:"text"`
	Annotations Annotations `json:"annotations"`
}

var (
	colorOpenRe = regexp.MustCompile(`^\{([a-zA-Z_]+):`)
	linkRe      = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s]+)\)`)
)

// ParseInline parses one line/run of extended markdown into Notion rich-text.
//
// Inline syntax:
//
//	**bold**  *italic*  ~~strike~~  `code`  <u>underline</u>
//	==highlight==                      -> yellow_background
//	{red:text}                         -> text color
//	{red_bg:text} / {red_background:text} -> background color
//	[label](https://url)               -> link (label may be formatted)
//	\{ \} \* \` \= \[ \\               -> escape a literal special char
//
// Color spans nest; the innermost color wins. The color field is single-valued,
// so an explicit {color:..} takes precedence over an enclosing == highlight.
func ParseInline(text string) []RichText {
	out := []RichText{}
	var bold, italic, strike, underline, highlight bool
	var colors []string
	var closeStack []byte // 'c' = a color span awaiting its '}'
	var buf strings.Builder

	curColor := func() string {
		if len(colors) > 0 {
			return colors[len(colors)-1]
		}
		return "default"
	}
	ann := func(code bool) Annotations {
		return Annotations{
			Bold: bold, Italic: italic, Strikethrough: strike,
			Underline: underline, Code: code, Color: curColor(),
		}
	}
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		out = append(out, RichText{Type: "text", Text: TextContent{Content: buf.String()}, Annotations: ann(false)})
		buf.Reset()
	}
	at := func(i int, p string) bool { return strings.HasPrefix(text[i:], p) }

	n := len(text)
	for i := 0; i < n; {
		c := text[i]

		if c == '\\' && i+1 < n { // escape
			buf.WriteByte(text[i+1])
			i += 2
			continue
		}
		if c == '`' { // code span (literal contents)
			if rel := strings.IndexByte(text[i+1:], '`'); rel >= 0 {
				flush()
				out = append(out, RichText{Type: "text", Text: TextContent{Content: text[i+1 : i+1+rel]}, Annotations: ann(true)})
				i = i + 1 + rel + 1
				continue
			}
		}
		if at(i, "**") {
			flush()
			bold = !bold
			i += 2
			continue
		}
		if at(i, "~~") {
			flush()
			strike = !strike
			i += 2
			continue
		}
		if at(i, "==") { // highlight via color stack
			flush()
			if highlight {
				for k := len(colors) - 1; k >= 0; k-- {
					if colors[k] == "yellow_background" {
						colors = append(colors[:k], colors[k+1:]...)
						break
					}
				}
				highlight = false
			} else {
				colors = append(colors, "yellow_background")
				highlight = true
			}
			i += 2
			continue
		}
		if at(i, "<u>") {
			flush()
			underline = true
			i += 3
			continue
		}
		if at(i, "</u>") {
			flush()
			underline = false
			i += 4
			continue
		}
		if c == '*' {
			flush()
			italic = !italic
			i++
			continue
		}
		if c == '{' {
			if m := colorOpenRe.FindStringSubmatch(text[i:]); m != nil {
				if col, ok := NormalizeColor(m[1]); ok {
					flush()
					colors = append(colors, col)
					closeStack = append(closeStack, 'c')
					i += len(m[0])
					continue
				}
			}
			buf.WriteByte(c)
			i++
			continue
		}
		if c == '}' {
			if len(closeStack) > 0 && closeStack[len(closeStack)-1] == 'c' {
				flush()
				closeStack = closeStack[:len(closeStack)-1]
				if len(colors) > 0 {
					colors = colors[:len(colors)-1]
				}
				i++
				continue
			}
			buf.WriteByte(c)
			i++
			continue
		}
		if c == '[' {
			if m := linkRe.FindStringSubmatch(text[i:]); m != nil {
				flush()
				url := m[2]
				for _, r := range ParseInline(m[1]) { // merge enclosing formatting
					r.Annotations.Bold = r.Annotations.Bold || bold
					r.Annotations.Italic = r.Annotations.Italic || italic
					r.Annotations.Strikethrough = r.Annotations.Strikethrough || strike
					r.Annotations.Underline = r.Annotations.Underline || underline
					if r.Annotations.Color == "default" {
						r.Annotations.Color = curColor()
					}
					r.Text.Link = &Link{URL: url}
					out = append(out, r)
				}
				i += len(m[0])
				continue
			}
		}
		buf.WriteByte(c)
		i++
	}
	flush()
	return splitLong(coalesce(out))
}

func sameAnnotations(a, b Annotations) bool { return a == b }

// coalesce merges adjacent runs with identical annotations and no link.
func coalesce(runs []RichText) []RichText {
	out := make([]RichText, 0, len(runs))
	for _, r := range runs {
		if r.Text.Content == "" {
			continue
		}
		if len(out) > 0 {
			last := &out[len(out)-1]
			if last.Text.Link == nil && r.Text.Link == nil && sameAnnotations(last.Annotations, r.Annotations) {
				last.Text.Content += r.Text.Content
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// splitLong breaks runs whose content exceeds Notion's 2000-char limit.
func splitLong(runs []RichText) []RichText {
	out := make([]RichText, 0, len(runs))
	for _, r := range runs {
		content := r.Text.Content
		if len(content) <= maxRichText {
			out = append(out, r)
			continue
		}
		for len(content) > 0 {
			end := maxRichText
			if end > len(content) {
				end = len(content)
			}
			// avoid splitting a multibyte rune
			for end > 0 && end < len(content) && content[end]&0xC0 == 0x80 {
				end--
			}
			chunk := content[:end]
			content = content[end:]
			out = append(out, RichText{Type: "text", Text: TextContent{Content: chunk, Link: r.Text.Link}, Annotations: r.Annotations})
		}
	}
	return out
}
