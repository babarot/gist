package keymap

import (
	"errors"
	"strings"
	"testing"
)

func TestEveryDefaultKeyIsOfOneAction(t *testing.T) {
	if _, err := Default().With(nil); err != nil {
		t.Fatal(err)
	}
}

func TestABoxMayShareAKeyWithTheList(t *testing.T) {
	m := Default()
	if !m.Matches(Rename, "r") || !m.Matches(Reload, "r") {
		t.Error("r is not both")
	}
	if _, err := m.With(map[string][]string{"overwrite": {"r"}}); err == nil {
		t.Error("two actions of a box took one key")
	}
}

func TestAKeyOfADiffIsNotAlsoOneThatSendsIt(t *testing.T) {
	over := map[string][]string{"page_down": {"y"}, "copy_url": {}}
	if _, err := Default().With(over); err == nil || !strings.Contains(err.Error(), "bound to both") {
		t.Errorf("got %v", err)
	}
}

func TestMatches(t *testing.T) {
	m := Default()
	if !m.Matches(Down, "j") || !m.Matches(Down, "down") || m.Matches(Down, "k") {
		t.Error("the defaults do not match")
	}
}

func TestConfigReplacesAnActionsKeys(t *testing.T) {
	m, err := Default().With(map[string][]string{"quit": {"Q", "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Matches(Quit, "Q") || m.Matches(Quit, "q") {
		t.Errorf("got %v", m.Keys(Quit))
	}
	if !Default().Matches(Quit, "q") {
		t.Error("the defaults changed")
	}
}

func TestMistakes(t *testing.T) {
	for _, tt := range []struct {
		over map[string][]string
		want string
	}{
		{map[string][]string{"qiut": {"Q"}}, `did you mean "quit"`},
		{map[string][]string{"nothing_like_it": {"Q"}}, "no action is named"},
		{map[string][]string{"quit": {"j"}}, "bound to both"},
		{map[string][]string{"quit": {"esc"}}, "fixed"},
		{map[string][]string{"quit": {"ctrl+c"}}, "fixed"},
	} {
		_, err := Default().With(tt.over)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%v: got %v", tt.over, err)
		}
	}
}

func TestHint(t *testing.T) {
	m := Default()
	if got := m.Hint(Down, Up); got != "j/k" {
		t.Errorf("got %q", got)
	}
	k, err := m.With(map[string][]string{"down": {"down"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := k.Hint(Down); got != "↓" {
		t.Errorf("got %q", got)
	}
	u, err := m.With(map[string][]string{"browse": {}})
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Hint(Browse); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestEveryMistakeIsGiven(t *testing.T) {
	_, err := Default().With(map[string][]string{
		"qiut":   {"q"},
		"help":   {"esc"},
		"browse": {"x", "J"}, // the key of scroll_down, which the file does not set
	})
	var ps Problems
	if !errors.As(err, &ps) || len(ps) != 3 {
		t.Fatalf("got %v", err)
	}
	for _, p := range ps {
		switch p.Action {
		case "qiut":
			if !p.AtKey || !strings.Contains(p.Message, `did you mean "quit"`) {
				t.Errorf("got %+v", p)
			}
		case "help":
			if p.Index != 0 {
				t.Errorf("got %+v", p)
			}
		case "browse":
			if p.Index != 1 {
				t.Errorf("got %+v", p)
			}
		default:
			t.Errorf("placed where the file does not set it: %+v", p)
		}
	}
}
