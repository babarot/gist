package config

import (
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
		`theme = "lattee"`:                "did you mean \"latte\"",
		`nope = 1`:                        "nope",
		"[new]\nvisibility = \"private\"": "new.visibility must be one of",
		"theme =":                         "config.toml",
	} {
		_, err := Load(write(t, content))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v", content, err)
		}
	}
}
