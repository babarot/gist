// Package keymap binds keys to actions by name. The TUI never writes a key in
// its code: it asks for an action, and draws the key hints and the ? list from
// here, so every key can be changed under [keys] in the config file.
//
// Keys are written as Bubble Tea reports key presses: "a", "A", "enter",
// "ctrl+s", "shift+tab".
package keymap

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/babarot/gist/v2/internal/near"
)

// Action is something a key does, named as it is under [keys].
type Action string

// The actions.
const (
	Quit          Action = "quit"
	Help          Action = "help"
	Refresh       Action = "refresh"
	Up            Action = "up"
	Down          Action = "down"
	Top           Action = "top"
	Bottom        Action = "bottom"
	PageUp        Action = "page_up"
	PageDown      Action = "page_down"
	PrevFile      Action = "prev_file"
	NextFile      Action = "next_file"
	Mine          Action = "tab_mine"
	Starred       Action = "tab_starred"
	Browse        Action = "browse"
	CopyURL       Action = "copy_url"
	CopyFile      Action = "copy_file"
	ScrollDown    Action = "scroll_down"
	ScrollUp      Action = "scroll_up"
	ToggleSide    Action = "toggle_comments"
	ToggleMD      Action = "toggle_markdown"
	PreviewGrow   Action = "preview_grow"
	PreviewShrink Action = "preview_shrink"
	ReleaseNotes  Action = "release_notes"
	Edit          Action = "edit"
	Delete        Action = "delete"
	Describe      Action = "edit_description"
	Rename        Action = "rename"
	Discard       Action = "discard_draft"
	Confirm       Action = "confirm"
	Overwrite     Action = "overwrite"
	Reload        Action = "reload"
	OnlyFile      Action = "only_file"
	EditAgain     Action = "edit_again"
	Filter        Action = "filter"
	New           Action = "new"
	Create        Action = "create"
	OpenEditor    Action = "open_editor"
	// Select, SelectShown and ClearSelection are named as gh-desk names them.
	Select         Action = "select"
	SelectShown    Action = "select_shown"
	ClearSelection Action = "clear_selection"
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
var boxActions = []Action{Confirm, Overwrite, Reload, OnlyFile, EditAgain, Create, OpenEditor, ReleaseNotes}

// sharedActions are read on the list and in the box of a diff, so their keys
// must not be those of what that box does.
var sharedActions = []Action{Up, Down, PageUp, PageDown, ScrollUp, ScrollDown}

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
}

// Defaults are every action with its keys.
var Defaults = []Def{
	{Quit, []string{"q"}},
	{Help, []string{"?"}},
	{Refresh, []string{"R"}},
	{Up, []string{"k", "up"}},
	{Down, []string{"j", "down"}},
	{Top, []string{"g", "home"}},
	{Bottom, []string{"G", "end"}},
	{PageUp, []string{"pgup", "ctrl+u"}},
	{PageDown, []string{"pgdown", "ctrl+d"}},
	{PrevFile, []string{"h", "left", "["}},
	{NextFile, []string{"l", "right", "]"}},
	{Mine, []string{"1"}},
	{Starred, []string{"2"}},
	{Browse, []string{"o"}},
	{CopyURL, []string{"y"}},
	{CopyFile, []string{"Y"}},
	{ScrollDown, []string{"J"}},
	{ScrollUp, []string{"K"}},
	{ToggleSide, []string{"c"}},
	{ToggleMD, []string{"m"}},
	{PreviewGrow, []string{"+"}},
	{PreviewShrink, []string{"-"}},
	{ReleaseNotes, []string{"w"}},
	{Edit, []string{"enter", "e"}},
	{Delete, []string{"D"}},
	{Describe, []string{"E"}},
	{Rename, []string{"r"}},
	{Discard, []string{"X"}},
	{Confirm, []string{"y"}},
	{Overwrite, []string{"o"}},
	{Reload, []string{"r"}},
	{OnlyFile, []string{"f"}},
	{EditAgain, []string{"e"}},
	{Filter, []string{"/"}},
	{New, []string{"n"}},
	{Create, []string{"ctrl+s"}},
	{OpenEditor, []string{"ctrl+e"}},
	{Select, []string{"space"}},
	{SelectShown, []string{"V"}},
	{ClearSelection, []string{"u"}},
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
}

// Default is the map with every action at its default keys.
func Default() Map {
	m := Map{keys: map[Action][]string{}}
	for _, d := range Defaults {
		m.keys[d.Action] = slices.Clone(d.Keys)
	}
	return m
}

// Problem is a mistake in [keys]: the action it is at, which of its keys or -1
// for the whole setting, and whether it is the name that is wrong.
type Problem struct {
	Action  string
	Index   int
	AtKey   bool
	Message string
}

// Problems are every mistake in [keys].
type Problems []Problem

func (ps Problems) Error() string {
	msgs := make([]string, len(ps))
	for i, p := range ps {
		msgs[i] = p.Message
	}
	return strings.Join(msgs, "\n")
}

// With is m with the keys of the config file, each action's replacing its
// defaults. A name that is no action, and a key two actions want, are mistakes:
// the error is Problems, with all of them.
func (m Map) With(over map[string][]string) (Map, error) {
	out := Map{keys: map[Action][]string{}}
	for a, k := range m.keys {
		out.keys[a] = slices.Clone(k)
	}
	names := make([]string, 0, len(m.keys))
	for a := range m.keys {
		names = append(names, string(a))
	}
	slices.Sort(names)
	var ps Problems
	for _, name := range slices.Sorted(maps.Keys(over)) {
		keys := over[name]
		if !slices.Contains(names, name) {
			msg := fmt.Sprintf("keys: no action is named %q", name)
			if n, ok := near.Closest(name, names); ok {
				msg += fmt.Sprintf("; did you mean %q?", n)
			}
			ps = append(ps, Problem{Action: name, Index: -1, AtKey: true, Message: msg})
			continue
		}
		for i, k := range keys {
			if k == Interrupt || k == Back {
				ps = append(ps, Problem{Action: name, Index: i, Message: fmt.Sprintf("keys: %s is fixed and cannot be bound to %s", k, name)})
			}
		}
		out.keys[Action(name)] = slices.Clone(keys)
	}
	bound := map[Scope]map[string]Action{ScopeList: {}, ScopeBox: {}}
	for _, name := range names {
		scopes := []Scope{ScopeOf(Action(name))}
		if slices.Contains(sharedActions, Action(name)) {
			scopes = append(scopes, ScopeBox)
		}
		for _, k := range out.keys[Action(name)] {
			clash := false
			for _, sc := range scopes {
				if other, ok := bound[sc][k]; ok && !clash {
					clash = true
					ps = append(ps, conflict(over, k, string(other), name))
				}
				bound[sc][k] = Action(name)
			}
		}
	}
	if len(ps) > 0 {
		return m, ps
	}
	return out, nil
}

// conflict is k wanted by two actions, placed at the one the file sets: the
// other may be at its default, which is nowhere in the file.
func conflict(over map[string][]string, k, first, second string) Problem {
	at := second
	if _, set := over[second]; !set {
		at = first
	}
	return Problem{Action: at, Index: slices.Index(over[at], k), Message: fmt.Sprintf("keys: %s is bound to both %s and %s", k, first, second)}
}

// Matches says key is one of action's keys.
func (m Map) Matches(action Action, key string) bool { return slices.Contains(m.keys[action], key) }

// Keys are action's keys.
func (m Map) Keys(action Action) []string { return m.keys[action] }

// glyphs are how key names show.
var glyphs = map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→", "pgup": "PgUp", "pgdown": "PgDn"}

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
