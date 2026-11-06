package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/babarot/gist/v2/internal/config"
)

// palette is the colors of a theme. Everything drawn takes its colors from
// here, so that a color cannot stay the same on the other theme.
type palette struct {
	dark                                         bool
	base, surface0, surface1, overlay            color.Color
	subtext0, subtext1, text, dim                color.Color
	blue, mauve, green, yellow, peach, red, teal color.Color
	pick, fresh                                  color.Color
}

// themes are the two color schemes: default, which is Catppuccin Mocha made
// lighter so that surfaces stand out on a terminal, and latte.
var themes = map[string]palette{
	config.ThemeDefault: {
		dark: true,
		base: lipgloss.Color("#1e1e2e"), surface0: lipgloss.Color("#3b3b52"), surface1: lipgloss.Color("#525270"),
		overlay: lipgloss.Color("#8888a4"), subtext0: lipgloss.Color("#b0b8d1"), subtext1: lipgloss.Color("#d0d6e8"),
		text: lipgloss.Color("#e0e4f0"), dim: lipgloss.Color("#706f87"),
		blue: lipgloss.Color("#96bfff"), mauve: lipgloss.Color("#d4b0ff"), green: lipgloss.Color("#b5f0b0"),
		yellow: lipgloss.Color("#ffe5a0"), peach: lipgloss.Color("#ffc49a"), red: lipgloss.Color("#f5a3b5"), teal: lipgloss.Color("#94e2d5"),
		pick: lipgloss.Color("#3a3150"), fresh: lipgloss.Color("#26324a"),
	},
	config.ThemeLatte: {
		base: lipgloss.Color("#eff1f5"), surface0: lipgloss.Color("#ccd0da"), surface1: lipgloss.Color("#bcc0cc"),
		overlay: lipgloss.Color("#7c7f93"), subtext0: lipgloss.Color("#6c6f85"), subtext1: lipgloss.Color("#5c5f77"),
		text: lipgloss.Color("#4c4f69"), dim: lipgloss.Color("#9ca0b0"),
		blue: lipgloss.Color("#1e66f5"), mauve: lipgloss.Color("#8839ef"), green: lipgloss.Color("#40a02b"),
		yellow: lipgloss.Color("#df8e1d"), peach: lipgloss.Color("#fe640b"), red: lipgloss.Color("#d20f39"), teal: lipgloss.Color("#179299"),
		pick: lipgloss.Color("#e6dcf7"), fresh: lipgloss.Color("#dce6fb"),
	},
}
