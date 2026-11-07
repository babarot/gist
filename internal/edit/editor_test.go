package edit

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestCommand(t *testing.T) {
	for _, tt := range []struct {
		name, configured, visual, editor string
		want                             []string
	}{
		{"nothing set", "", "", "", []string{"vi", "f"}},
		{"$EDITOR", "", "", "nano", []string{"nano", "f"}},
		{"$VISUAL first", "", "code -w", "nano", []string{"code", "-w", "f"}},
		{"the config first", "hx", "code -w", "nano", []string{"hx", "f"}},
		{"spaces are nothing", "  ", "", "", []string{"vi", "f"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("VISUAL", tt.visual)
			t.Setenv("EDITOR", tt.editor)
			if got := Command(tt.configured, "f"); !slices.Equal(got, tt.want) {
				t.Errorf("got %v", got)
			}
		})
	}
}

func TestWorkspaceRoundTrip(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.go": raw, "b.md": "b"})
	es := begin(t, e, g.ID, "a.go", "b.md")
	w, err := es.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	paths := w.Paths()
	if filepath.Base(paths[0]) != "a.go" || filepath.Base(paths[1]) != "b.md" {
		t.Errorf("paths are %v", paths)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil || string(b) != raw {
		t.Errorf("got %q, %v", b, err)
	}
	if fi, err := os.Stat(filepath.Dir(paths[0])); err != nil || runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
		t.Errorf("the directory is %v, %v", fi.Mode(), err)
	}
	if err := os.WriteFile(paths[1], []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := w.ReadBack(es)
	if err != nil || !changed {
		t.Fatalf("changed %v, %v", changed, err)
	}
	if _, err := os.Stat(filepath.Dir(paths[0])); !os.IsNotExist(err) {
		t.Errorf("the files stayed: %v", err)
	}
	if d, ok, _ := e.Draft(g.ID, "b.md"); !ok || string(d.Content) != "edited" {
		t.Errorf("no draft: %+v", d)
	}
	if _, ok, _ := e.Draft(g.ID, "a.go"); ok {
		t.Error("a draft of a file left as it was")
	}
}

func TestReadBackOfNoChange(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.go": raw})
	es := begin(t, e, g.ID, "a.go")
	w, err := es.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := w.ReadBack(es); err != nil || changed {
		t.Errorf("changed %v, %v", changed, err)
	}
}

func TestAnEditThatCannotBeKeptStaysOnDisk(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.go": "a"})
	es := begin(t, e, g.ID, "a.go")
	w, err := es.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.Paths()[0], []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A file where the gist's drafts should go
	drafts := filepath.Join(stateDirOf(t, e), "drafts")
	if err := os.WriteFile(drafts, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = w.ReadBack(es)
	var kept *KeptAtError
	if !errors.As(err, &kept) {
		t.Fatalf("got %v", err)
	}
	if b, rerr := os.ReadFile(filepath.Join(kept.Dir, "a.go")); rerr != nil || string(b) != "edited" {
		t.Errorf("the edit is gone: %q, %v", b, rerr)
	}
}

func TestARemovedFileLeavesTheEdit(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.go": "a"})
	es := begin(t, e, g.ID, "a.go")
	w, err := es.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(w.Paths()[0]); err != nil {
		t.Fatal(err)
	}
	if changed, err := w.ReadBack(es); err != nil || changed {
		t.Errorf("changed %v, %v", changed, err)
	}
}

func TestScratch(t *testing.T) {
	s, err := NewScratch("a.go", "first")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(s.Path()) != "a.go" {
		t.Errorf("path is %s", s.Path())
	}
	if err := os.WriteFile(s.Path(), []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read()
	if err != nil || got != "second" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Errorf("the file stayed: %v", err)
	}
}

func TestNamesThatDifferOnlyInCaseKeepTheirContent(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"A.go": "upper\n", "a.go": "lower\n", "b.go": "other\n"})
	es := begin(t, e, g.ID, "A.go", "a.go", "b.go")
	w, err := es.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for i, p := range w.Paths() {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		want[es.Files[i].Name] = string(b)
		if err := os.WriteFile(p, append(b, "more\n"...), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if want["A.go"] != "upper\n" || want["a.go"] != "lower\n" || want["b.go"] != "other\n" {
		t.Fatalf("written as %q", want)
	}
	if changed, err := w.ReadBack(es); err != nil || !changed {
		t.Fatalf("changed %v, %v", changed, err)
	}
	for name, content := range want {
		if got := es.File(name).Content; got != content+"more\n" {
			t.Errorf("%s is %q", name, got)
		}
		d, ok, err := e.Draft(g.ID, name)
		if err != nil || !ok || d.File != name || string(d.Content) != content+"more\n" {
			t.Errorf("the draft of %s is %+v, %v, %v", name, d, ok, err)
		}
	}
}
