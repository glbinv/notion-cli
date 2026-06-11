// Package notionmd converts an extended-markdown dialect into Notion block
// objects, preserving the color and annotation fidelity that the official
// Notion MCP and thin SDK wrappers drop. It is the formatting-first core of
// notion-pp-cli's writer commands.
package notionmd

import "strings"

// TextColors are Notion's foreground annotation colors. Each (except "default")
// also has a "<color>_background" variant.
var TextColors = []string{
	"default", "gray", "brown", "orange", "yellow",
	"green", "blue", "purple", "pink", "red",
}

var colorSet = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range TextColors {
		m[c] = true
		if c != "default" {
			m[c+"_background"] = true
		}
	}
	return m
}()

// IsColor reports whether s is a valid Notion annotation/block color.
func IsColor(s string) bool { return colorSet[s] }

// NormalizeColor maps a user token to a valid Notion color, or ("", false).
// Accepts aliases: grey→gray, `_bg`→`_background`, `bg_red`/`background_red`→
// red_background, and the keyword `highlight`→yellow_background.
func NormalizeColor(input string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)
	switch s {
	case "highlight":
		s = "yellow_background"
	case "grey":
		s = "gray"
	case "grey_background":
		s = "gray_background"
	}
	if rest, ok := strings.CutPrefix(s, "bg_"); ok {
		s = rest + "_background"
	} else if rest, ok := strings.CutPrefix(s, "background_"); ok {
		s = rest + "_background"
	}
	if rest, ok := strings.CutSuffix(s, "_bg"); ok {
		s = rest + "_background"
	}
	if s == "grey_background" {
		s = "gray_background"
	}
	if colorSet[s] {
		return s, true
	}
	return "", false
}
