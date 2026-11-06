// Package keymap binds keys to actions by name. The TUI never writes a key in
// its code: it asks for an action, and draws the key hints and the ? list from
// here, so every key can be changed under [keys] in the config file.
//
// Keys are written as Bubble Tea reports key presses: "a", "A", "enter",
// "ctrl+s", "shift+tab".
package keymap

import (
	"fmt"
	"slices"
	"strings"

	"github.com/babarot/gist/v2/internal/near"
)

// Action is something a key does, named as it is under [keys].
type Action string

// The actions.
const (
	Quit       Action = "quit"
	Help       Action = "help"
	Refresh    Action = "refresh"
	Up         Action = "up"
	Down       Action = "down"
	Top        Action = "top"
	Bottom     Action = "bottom"
	PrevFile   Action = "prev_file"
	NextFile   Action = "next_file"
	Mine       Action = "tab_mine"
	Starred    Action = "tab_starred"
	Browse     Action = "browse"
	CopyURL    Action = "copy_url"
	ScrollDown Action = "scroll_down"
	ScrollUp   Action = "scroll_up"
	ToggleSide Action = "toggle_comments"
	ToggleMD   Action = "toggle_markdown"
	Edit       Action = "edit"
	Delete     Action = "delete"
	Describe   Action = "edit_description"
	Rename     Action = "rename"
	Discard    Action = "discard_draft"
	Confirm    Action = "confirm"
	Overwrite  Action = "overwrite"
	Reload     Action = "reload"
	OnlyFile   Action = "only_file"
	EditAgain  Action = "edit_again"
)

// Scope is where an action's keys are read. Two actions may share a key only
// when they are read in different scopes.
type Scope int

// The scopes.
const (
	ScopeList Scope = iota
	// ScopeBox is read in what is open over the screen.
	ScopeBox
)

// boxActions are read over the screen, not on the list.
var boxActions = []Action{Confirm, Overwrite, Reload, OnlyFile, EditAgain}

// ScopeOf is where action's keys are read.
func ScopeOf(a Action) Scope {
	if slices.Contains(boxActions, a) {
		return ScopeBox
	}
	return ScopeList
}

// Def is an action with its default keys and the words the key hints show.
type Def struct {
	Action Action
	Keys   []string
	Help   string
}

// Defaults are every action with its keys.
var Defaults = []Def{
	{Quit, []string{"q"}, "quit"},
	{Help, []string{"?"}, "keys"},
	{Refresh, []string{"R"}, "refresh"},
	{Up, []string{"k", "up"}, "up"},
	{Down, []string{"j", "down"}, "down"},
	{Top, []string{"g", "home"}, "top"},
	{Bottom, []string{"G", "end"}, "bottom"},
	{PrevFile, []string{"h", "left", "["}, "previous file"},
	{NextFile, []string{"l", "right", "]"}, "next file"},
	{Mine, []string{"1"}, "mine"},
	{Starred, []string{"2"}, "starred"},
	{Browse, []string{"o"}, "browser"},
	{CopyURL, []string{"y"}, "copy url"},
	{ScrollDown, []string{"J"}, "scroll down"},
	{ScrollUp, []string{"K"}, "scroll up"},
	{ToggleSide, []string{"c"}, "comments"},
	{ToggleMD, []string{"m"}, "markdown"},
	{Edit, []string{"enter", "e"}, "edit"},
	{Delete, []string{"D"}, "delete"},
	{Describe, []string{"E"}, "desc"},
	{Rename, []string{"r"}, "rename"},
	{Discard, []string{"X"}, "discard draft"},
	{Confirm, []string{"y"}, "yes"},
	{Overwrite, []string{"o"}, "overwrite"},
	{Reload, []string{"r"}, "reload"},
	{OnlyFile, []string{"f"}, "only this file"},
	{EditAgain, []string{"e"}, "edit again"},
}

// The fixed keys are read before any action and cannot be bound.
const (
	// Interrupt quits from anywhere.
	Interrupt = "ctrl+c"
	// Back closes what is open.
	Back = "esc"
)

// Map is the keys of every action, after the config file.
type Map struct {
	keys map[Action][]string
	help map[Action]string
}

// Default is the map with every action at its default keys.
func Default() Map {
	m := Map{keys: map[Action][]string{}, help: map[Action]string{}}
	for _, d := range Defaults {
		m.keys[d.Action] = slices.Clone(d.Keys)
		m.help[d.Action] = d.Help
	}
	return m
}

// With is m with the keys of the config file, each action's replacing its
// defaults. A name that is no action, and a key two actions want, are mistakes.
func (m Map) With(over map[string][]string) (Map, error) {
	out := Map{keys: map[Action][]string{}, help: m.help}
	for a, k := range m.keys {
		out.keys[a] = slices.Clone(k)
	}
	names := make([]string, 0, len(m.keys))
	for a := range m.keys {
		names = append(names, string(a))
	}
	slices.Sort(names)
	for name, keys := range over {
		if !slices.Contains(names, name) {
			msg := fmt.Sprintf("keys: no action is named %q", name)
			if n, ok := near.Closest(name, names); ok {
				msg += fmt.Sprintf("; did you mean %q?", n)
			}
			return m, fmt.Errorf("%s", msg)
		}
		for _, k := range keys {
			if k == Interrupt || k == Back {
				return m, fmt.Errorf("keys: %s is fixed and cannot be bound to %s", k, name)
			}
		}
		out.keys[Action(name)] = slices.Clone(keys)
	}
	bound := map[Scope]map[string]Action{ScopeList: {}, ScopeBox: {}}
	for _, name := range names {
		scope := bound[ScopeOf(Action(name))]
		for _, k := range out.keys[Action(name)] {
			if other, ok := scope[k]; ok {
				return m, fmt.Errorf("keys: %s is bound to both %s and %s", k, other, name)
			}
			scope[k] = Action(name)
		}
	}
	return out, nil
}

// Matches says key is one of action's keys.
func (m Map) Matches(action Action, key string) bool { return slices.Contains(m.keys[action], key) }

// Keys are action's keys.
func (m Map) Keys(action Action) []string { return m.keys[action] }

// HelpOf is the words of action.
func (m Map) HelpOf(action Action) string { return m.help[action] }

// glyphs are how key names show.
var glyphs = map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→"}

// Hint is how action's keys show: its first, or its first of several joined,
// each as a person reads it. "" for an action with no keys.
func (m Map) Hint(actions ...Action) string {
	var parts []string
	for _, a := range actions {
		if keys := m.keys[a]; len(keys) > 0 {
			parts = append(parts, Glyph(keys[0]))
		}
	}
	return strings.Join(parts, "/")
}

// Glyph is how a key shows.
func Glyph(key string) string {
	if g, ok := glyphs[key]; ok {
		return g
	}
	return key
}
