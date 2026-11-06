package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMissingFileIsTheDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil || !reflect.DeepEqual(got, Default()) {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestTemplateIsTheDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	if err := WriteTemplate(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, Default()) {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestTemplateIsNotWrittenOverAFile(t *testing.T) {
	path := write(t, `theme = "latte"`)
	if err := WriteTemplate(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Theme != ThemeLatte {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestSettings(t *testing.T) {
	got, err := Load(write(t, `
editor = "code -w"
theme = "latte"
[new]
visibility = "public"
[list]
lead = "description"
[preview]
markdown = "plain"
[keys]
quit = ["Q"]
`))
	if err != nil {
		t.Fatal(err)
	}
	want := File{"code -w", ThemeLatte, New{"public"}, List{"description"}, Preview{"plain"}, map[string][]string{"quit": {"Q"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestMistakes(t *testing.T) {
	for content, want := range map[string]string{
		`theme = "lattee"`:                "config.toml:1:9: theme must be one of auto, default, latte, got \"lattee\"; did you mean \"latte\"?",
		`themes = "latte"`:                "config.toml:1:1: themes is not a setting; did you mean \"theme\"?",
		"[new]\nvisibility = \"private\"": "config.toml:2:14: new.visibility must be one of",
		"[lists]\n":                       "config.toml:1:2: lists is not a setting; did you mean \"list\"?",
		"theme =":                         "config.toml:1:",
	} {
		path := write(t, content)
		_, err := Load(path)
		var ps Problems
		if !errors.As(err, &ps) {
			t.Fatalf("%q: got %v", content, err)
		}
		if got := Report(path, ps); !strings.Contains(got, want) {
			t.Errorf("%q: got\n%s", content, got)
		}
	}
}

func TestEveryMistakeIsReported(t *testing.T) {
	path := write(t, "themes = \"auto\"\n[list]\nlead = \"nope\"\n[preview]\nmarkdown = \"nope\"\n")
	_, err := Load(path)
	var ps Problems
	if !errors.As(err, &ps) || len(ps) != 3 {
		t.Fatalf("got %v", err)
	}
	got := Report(path, ps)
	for _, want := range []string{"config.toml:1:1:", "config.toml:3:8:", "config.toml:5:12:"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in\n%s", want, got)
		}
	}
	// Top to bottom, with the line and the mistake marked
	if !strings.Contains(got, "3 | lead = \"nope\"\n  |        ^~~~~~") {
		t.Errorf("got\n%s", got)
	}
}

func TestReportMarksAnElementOfAList(t *testing.T) {
	path := write(t, "[keys]\nquit = [\"q\", \"esc\"]\n")
	got := Report(path, Problems{{Key: []string{"keys", "quit"}, Index: 1, Message: "esc is fixed"}})
	if !strings.Contains(got, "config.toml:2:14: esc is fixed") {
		t.Errorf("got\n%s", got)
	}
}

func TestReportWithoutAPlace(t *testing.T) {
	path := write(t, "")
	got := Report(path, Problems{{Key: []string{"nowhere"}, Index: -1, Message: "lost"}})
	if !strings.HasSuffix(got, "config.toml: lost") {
		t.Errorf("got %q", got)
	}
}
