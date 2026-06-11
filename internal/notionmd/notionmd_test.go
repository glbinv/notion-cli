package notionmd

import "testing"

func TestNormalizeColor(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"red":            {"red", true},
		"RED":            {"red", true},
		"red_bg":         {"red_background", true},
		"red background": {"red_background", true},
		"bg_red":         {"red_background", true},
		"grey":           {"gray", true},
		"highlight":      {"yellow_background", true},
		"red_background": {"red_background", true},
		"mauve":          {"", false},
	}
	for in, want := range cases {
		got, ok := NormalizeColor(in)
		if ok != want.ok || got != want.want {
			t.Errorf("NormalizeColor(%q) = (%q,%v), want (%q,%v)", in, got, ok, want.want, want.ok)
		}
	}
}

func TestParseInlineMixedAnnotations(t *testing.T) {
	runs := ParseInline("This is **{red:critical}** and ==flagged== and `code`.")
	// Expect a bold+red run, a yellow_background run, and a code run.
	var sawBoldRed, sawHighlight, sawCode bool
	for _, r := range runs {
		if r.Text.Content == "critical" && r.Annotations.Bold && r.Annotations.Color == "red" {
			sawBoldRed = true
		}
		if r.Text.Content == "flagged" && r.Annotations.Color == "yellow_background" {
			sawHighlight = true
		}
		if r.Text.Content == "code" && r.Annotations.Code {
			sawCode = true
		}
	}
	if !sawBoldRed {
		t.Error("expected a bold+red run for {red:critical} inside **..**")
	}
	if !sawHighlight {
		t.Error("expected ==flagged== to map to yellow_background")
	}
	if !sawCode {
		t.Error("expected `code` to be a code run")
	}
}

func TestParseInlineCodeSpanIsLiteral(t *testing.T) {
	runs := ParseInline("`{blue:x}`")
	if len(runs) != 1 || runs[0].Text.Content != "{blue:x}" || !runs[0].Annotations.Code {
		t.Errorf("code span should be literal, got %+v", runs)
	}
}

func TestParseMarkdownBlocks(t *testing.T) {
	blocks := ParseMarkdown("# Title {color=blue}\n\n> [!callout] {bg=red icon=rocket} Heads up\n")
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "heading_1" {
		t.Errorf("expected heading_1, got %v", blocks[0]["type"])
	}
	h := blocks[0]["heading_1"].(map[string]any)
	if h["color"] != "blue" {
		t.Errorf("expected heading color blue, got %v", h["color"])
	}
	if blocks[1]["type"] != "callout" {
		t.Fatalf("expected callout, got %v", blocks[1]["type"])
	}
	co := blocks[1]["callout"].(map[string]any)
	if co["color"] != "red_background" {
		t.Errorf("expected callout color red_background, got %v", co["color"])
	}
	icon := co["icon"].(map[string]any)
	if icon["emoji"] != "🚀" {
		t.Errorf("expected rocket emoji, got %v", icon["emoji"])
	}
}
