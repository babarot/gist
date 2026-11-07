package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/state"
	"github.com/babarot/gist/v2/internal/xdg"
)

// raw is a file only the raw content keeps as written: a tab, a CR, an escape
// sequence, and a character with a variation selector.
const raw = "col\tumn\r\n\x1b[31mred\x1b[0m\n❤️\n"

// env is a run of gist against a made-up GitHub.
type env struct {
	t     *testing.T
	s     *fakegist.Server
	stdin string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	setHome(t)
	t.Setenv("GH_TOKEN", "test-token")
	return &env{t: t, s: fakegist.New(t)}
}

// run runs gist with args, with what is in e.stdin.
func (e *env) run(args ...string) (stdout, stderr string, code int) {
	e.t.Helper()
	var out, errOut bytes.Buffer
	o := &options{newClient: func(log *slog.Logger, token string) (*github.Client, error) {
		return github.New(log, github.Options{Token: token, Transport: e.s.Transport()})
	}}
	code = run(o, args, strings.NewReader(e.stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// ok runs gist and fails the test unless it succeeds.
func (e *env) ok(args ...string) (stdout, stderr string) {
	e.t.Helper()
	stdout, stderr, code := e.run(args...)
	if code != 0 {
		e.t.Fatalf("gist %v: exit %d, stderr %q", args, code, stderr)
	}
	return stdout, stderr
}

// fails runs gist and fails the test unless it exits with code and says want.
func (e *env) fails(code int, want string, args ...string) {
	e.t.Helper()
	stdout, stderr, got := e.run(args...)
	if got != code {
		e.t.Errorf("gist %v: exit %d, want %d; stderr %q", args, got, code, stderr)
	}
	if stdout != "" {
		e.t.Errorf("gist %v: stdout is %q, want nothing", args, stdout)
	}
	if !strings.Contains(stderr, want) {
		e.t.Errorf("gist %v: stderr is %q, want %q", args, stderr, want)
	}
}

func (e *env) file(id, name string) string {
	e.t.Helper()
	g := e.s.Get(id)
	if g == nil {
		e.t.Fatalf("no gist %s", id)
	}
	return g.Revisions[0].Files[name]
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListWithoutATerminal(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("a gist", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	e.s.Comment(g.ID, "bob", "hi")
	stdout, _ := e.ok("list")
	want := fmt.Sprintf("%s\ta gist\t2\t1\tsecret\t%s\n", g.ID, e.s.Get(g.ID).Updated.Format("2006-01-02T15:04:05Z07:00"))
	if stdout != want {
		t.Errorf("got %q, want %q", stdout, want)
	}
	if other, _ := e.ok("ls"); other != stdout {
		t.Errorf("ls gave %q", other)
	}
	if bare, _ := e.ok(); bare != stdout {
		t.Errorf("gist in a pipe gave %q", bare)
	}
}

func TestListKeepsToOneLinePerGist(t *testing.T) {
	e := newEnv(t)
	e.s.Add("two\nlines\tand \x1b[2Jescape", false, map[string]string{"a.txt": "a"})
	stdout, _ := e.ok("list")
	if strings.Count(stdout, "\n") != 1 || strings.Count(stdout, "\t") != 5 || strings.Contains(stdout, "\x1b") {
		t.Errorf("got %q", stdout)
	}
}

func TestListFilters(t *testing.T) {
	e := newEnv(t)
	e.s.Add("pub", true, map[string]string{"a.txt": "a"})
	e.s.Add("sec1", false, map[string]string{"a.txt": "a"})
	e.s.Add("sec2", false, map[string]string{"a.txt": "a"})
	for _, tt := range []struct {
		args []string
		want []string
	}{
		{[]string{"--public"}, []string{"pub"}},
		{[]string{"--secret"}, []string{"sec2", "sec1"}},
		{[]string{"--limit", "1"}, []string{"sec2"}},
		{[]string{"--secret", "--limit", "1"}, []string{"sec2"}},
	} {
		stdout, _ := e.ok(append([]string{"list"}, tt.args...)...)
		var got []string
		for line := range strings.Lines(stdout) {
			got = append(got, strings.Split(line, "\t")[1])
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%v: got %v", tt.args, got)
		}
	}
	e.fails(2, "none of the others", "list", "--public", "--secret")
}

func TestListJSON(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("<d>", true, map[string]string{"a.txt": "a"})
	stdout, _ := e.ok("list", "--json")
	var got []struct {
		ID, Description string
		Public          bool
		Files           []struct {
			Name    string
			Content string
		}
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("%v: %s", err, stdout)
	}
	if len(got) != 1 || got[0].ID != g.ID || got[0].Description != "<d>" || got[0].Files[0].Name != "a.txt" {
		t.Errorf("got %+v", got)
	}
}

func TestListStarred(t *testing.T) {
	e := newEnv(t)
	e.s.Add("mine", false, map[string]string{"a.txt": "a"})
	e.s.Star("bob", "theirs", map[string]string{"a.txt": "a"})
	stdout, _ := e.ok("list", "--starred")
	if !strings.Contains(stdout, "theirs") || strings.Contains(stdout, "mine") {
		t.Errorf("got %q", stdout)
	}
}

func TestListUsesTheCacheUntilRefreshed(t *testing.T) {
	e := newEnv(t)
	e.s.Add("first", false, map[string]string{"a.txt": "a"})
	e.ok("list")
	e.s.Add("made elsewhere", false, map[string]string{"a.txt": "a"})
	if stdout, _ := e.ok("list"); strings.Contains(stdout, "elsewhere") {
		t.Errorf("read GitHub: %q", stdout)
	}
	if stdout, _ := e.ok("list", "--refresh"); !strings.Contains(stdout, "elsewhere") {
		t.Errorf("not refreshed: %q", stdout)
	}
}

func TestRefreshReadsOnlyWhatChanged(t *testing.T) {
	e := newEnv(t)
	e.s.Add("a", false, map[string]string{"a.txt": "a"})
	e.ok("list")
	e.ok("list", "--refresh")
	// The list is read twice, the second time answered with a 304, which
	// keeps the gists it had
	stdout, _ := e.ok("list")
	if !strings.Contains(stdout, "\ta\t") {
		t.Errorf("got %q", stdout)
	}
}

func TestViewPrintsTheBytesAsWritten(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": raw})
	stdout, _ := e.ok("view", g.ID)
	if stdout != raw {
		t.Errorf("got %q, want %q", stdout, raw)
	}
	if other, _ := e.ok("cat", g.ID, "a.txt"); other != raw {
		t.Errorf("cat gave %q", other)
	}
}

func TestViewManyFiles(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	stdout, _ := e.ok("view", g.ID)
	if want := "==> a.txt <==\na\n\n==> b.txt <==\nb\n"; stdout != want {
		t.Errorf("got %q, want %q", stdout, want)
	}
	if stdout, _ := e.ok("view", g.ID, "b.txt"); stdout != "b\n" {
		t.Errorf("got %q", stdout)
	}
	if stdout, _ := e.ok("view", g.ID, "--files"); stdout != "a.txt\nb.txt\n" {
		t.Errorf("got %q", stdout)
	}
	e.fails(1, `no file "c.txt"`, "view", g.ID, "c.txt")
}

func TestViewJSONGivesTheBaseForAnEdit(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("d", false, map[string]string{"a.txt": raw})
	stdout, _ := e.ok("view", g.ID, "--json")
	var got struct {
		Revision string
		Files    []struct{ Content string }
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Revision != g.Revisions[0].Version || got.Files[0].Content != raw {
		t.Errorf("got %+v", got)
	}
}

func TestViewLargeFileIsWhole(t *testing.T) {
	e := newEnv(t)
	content := strings.Repeat("x", fakegist.MaxContent+10)
	g := e.s.Add("", false, map[string]string{"big.txt": content})
	if stdout, _ := e.ok("view", g.ID); stdout != content {
		t.Errorf("got %d bytes", len(stdout))
	}
}

func TestResolve(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"deadlink": "dead\n"})
	e.s.Add("", false, map[string]string{"00-index.md": "x", "a.txt": "a"})
	e.s.Add("", false, map[string]string{"00-index.md": "y", "b.txt": "b"})
	for _, arg := range []string{g.ID, g.ID[:6], "deadlink", "https://gist.github.com/" + g.ID, "https://gist.github.com/alice/" + g.ID} {
		if stdout, _ := e.ok("view", arg); stdout != "dead\n" {
			t.Errorf("%s: got %q", arg, stdout)
		}
	}
	e.fails(1, "matches 2 gists", "view", "00-index.md")
	e.fails(1, `no gist matches "nothing"`, "view", "nothing")
}

func TestResolveFindsAGistMadeElsewhere(t *testing.T) {
	e := newEnv(t)
	e.s.Add("", false, map[string]string{"a.txt": "a"})
	e.ok("list")
	e.s.Add("", false, map[string]string{"new.txt": "new\n"})
	if stdout, _ := e.ok("view", "new.txt"); stdout != "new\n" {
		t.Errorf("got %q", stdout)
	}
}

func TestNewFromFiles(t *testing.T) {
	e := newEnv(t)
	a, b := writeFile(t, "main.go", raw), writeFile(t, "README.md", "readme")
	stdout, _ := e.ok("new", "-d", "retry helper", a, b)
	id := strings.TrimSpace(stdout[strings.LastIndex(stdout, "/")+1:])
	g := e.s.Get(id)
	if g == nil || g.Public || g.Description != "retry helper" {
		t.Fatalf("got %+v from %q", g, stdout)
	}
	if e.file(id, "main.go") != raw || e.file(id, "README.md") != "readme" {
		t.Error("the files changed on the way")
	}
}

func TestNewVisibility(t *testing.T) {
	for _, tt := range []struct {
		flags  []string
		public bool
	}{{nil, false}, {[]string{"--public"}, true}, {[]string{"--secret"}, false}} {
		e := newEnv(t)
		stdout, _ := e.ok(append([]string{"new", writeFile(t, "a.txt", "a")}, tt.flags...)...)
		id := strings.TrimSpace(stdout[strings.LastIndex(stdout, "/")+1:])
		if got := e.s.Get(id).Public; got != tt.public {
			t.Errorf("%v: public is %v", tt.flags, got)
		}
	}
}

func TestNewFromStdin(t *testing.T) {
	e := newEnv(t)
	e.stdin = raw
	stdout, _ := e.ok("new", "-f", "out.log")
	id := strings.TrimSpace(stdout[strings.LastIndex(stdout, "/")+1:])
	if e.file(id, "out.log") != raw {
		t.Errorf("got %q", e.file(id, "out.log"))
	}
	stdout, _ = e.ok("new")
	id = strings.TrimSpace(stdout[strings.LastIndex(stdout, "/")+1:])
	if e.file(id, "gistfile1.txt") != raw {
		t.Errorf("no default name")
	}
}

func TestNewRefuses(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	e.fails(1, "is a directory", "new", dir)
	e.fails(1, "is empty", "new", writeFile(t, "a.txt", ""))
	e.fails(1, "is binary", "new", writeFile(t, "a.png", "\x89PNG\x00\xff"))
	e.fails(1, "is empty", "new")
	e.stdin = "x"
	e.fails(1, "--filename names what comes from stdin", "new", "-f", "n", writeFile(t, "a.txt", "a"))
	for _, r := range e.s.Requests() {
		if strings.HasPrefix(r, "POST") {
			t.Errorf("sent %s", r)
		}
	}
}

func TestNewJSON(t *testing.T) {
	e := newEnv(t)
	stdout, _ := e.ok("new", "--json", writeFile(t, "a.txt", "a"))
	var got struct {
		ID, URL string
		Files   []string
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || !strings.HasSuffix(got.URL, got.ID) || !slices.Equal(got.Files, []string{"a.txt"}) {
		t.Errorf("got %+v", got)
	}
}

func TestNewIsInTheKeptList(t *testing.T) {
	e := newEnv(t)
	e.s.Add("first", false, map[string]string{"a.txt": "a"})
	e.ok("list")
	e.ok("new", "-d", "second", writeFile(t, "b.txt", "b"))
	stdout, _ := e.ok("list")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "second") {
		t.Errorf("got %q; the newest first", stdout)
	}
}

func TestEditFromStdin(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	e.stdin = raw
	e.ok("edit", g.ID, "a.txt")
	if e.file(g.ID, "a.txt") != raw || e.file(g.ID, "b.txt") != "b" {
		t.Errorf("got %q", e.s.Get(g.ID).Revisions[0].Files)
	}
}

func TestEditFromStdinOfTheOnlyFile(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a"})
	e.stdin = "new"
	e.ok("edit", g.ID)
	if e.file(g.ID, "a.txt") != "new" {
		t.Errorf("got %q", e.file(g.ID, "a.txt"))
	}
}

func TestEditSameContentSendsNothing(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a"})
	e.stdin = "a"
	_, stderr := e.ok("edit", g.ID, "a.txt")
	if !strings.Contains(stderr, "nothing sent") {
		t.Errorf("stderr is %q", stderr)
	}
	if len(e.s.Get(g.ID).Revisions) != 1 {
		t.Error("made a revision")
	}
}

func TestEditRefuses(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	e.stdin = "new"
	e.fails(1, "has 2 files", "edit", g.ID)
	e.fails(1, `no file "c.txt"`, "edit", g.ID, "c.txt")
	e.stdin = ""
	e.fails(1, "is empty", "edit", g.ID, "a.txt")
	if _, ok := e.s.Get(g.ID).Revisions[0].Files["a.txt"]; !ok {
		t.Error("empty stdin removed the file")
	}
	e.stdin = "\x00binary"
	e.fails(1, "is binary", "edit", g.ID, "a.txt")
}

func TestEditDescription(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("old", false, map[string]string{"a.txt": "a"})
	e.ok("edit", g.ID, "-d", "new description")
	got := e.s.Get(g.ID)
	if got.Description != "new description" || len(got.Revisions) != 1 {
		t.Errorf("got %+v", got)
	}
	e.ok("edit", g.ID, "-d", "")
	if got := e.s.Get(g.ID).Description; got != "" {
		t.Errorf("an empty description: got %q", got)
	}
}

func TestEditFiles(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"old.md": "o", "a.md": "a", "keep.md": "k"})
	e.ok("edit", g.ID, "--add", writeFile(t, "notes.md", raw), "--remove", "old.md", "--rename", "a.md=b.md")
	got := e.s.Get(g.ID)
	if len(got.Revisions) != 2 {
		t.Errorf("%d revisions: it should be one PATCH", len(got.Revisions))
	}
	files := got.Revisions[0].Files
	if files["notes.md"] != raw || files["b.md"] != "a" || files["keep.md"] != "k" || len(files) != 3 {
		t.Errorf("files are %q", files)
	}
}

func TestEditFilesRefuses(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.md": "a", "b.md": "b"})
	e.fails(1, "already has b.md", "edit", g.ID, "--rename", "a.md=b.md")
	e.fails(1, `no file "c.md"`, "edit", g.ID, "--rename", "c.md=d.md")
	e.fails(1, `no file "c.md"`, "edit", g.ID, "--remove", "c.md")
	e.fails(1, "already has a.md", "edit", g.ID, "--add", writeFile(t, "a.md", "x"))
	e.fails(1, "--rename takes old=new", "edit", g.ID, "--rename", "a.md")
	e.fails(1, "both removed and renamed", "edit", g.ID, "--remove", "a.md", "--rename", "a.md=c.md")
	for _, r := range e.s.Requests() {
		if strings.HasPrefix(r, "PATCH") {
			t.Errorf("sent %s", r)
		}
	}
}

func TestEditDropsNothingItWasPiped(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("old", false, map[string]string{"a.txt": "a"})
	e.stdin = "new content"
	e.fails(1, "a file name goes with piped content", "edit", g.ID, "a.txt", "-d", "new")
	if got := e.s.Get(g.ID).Description; got != "old" {
		t.Errorf("the description changed: %q", got)
	}
}

func TestEditRefusesTwoFilesOfOneName(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.md": "a", "b.md": "b"})
	e.fails(1, "two files would be named c.md", "edit", g.ID, "--rename", "a.md=c.md", "--rename", "b.md=c.md")
	e.fails(1, "two files are named x.md", "edit", g.ID, "--add", writeFile(t, "x.md", "1"), "--add", writeFile(t, "x.md", "2"))
	if len(e.s.Get(g.ID).Revisions) != 1 {
		t.Error("sent something")
	}
}

func TestEditWithABase(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a"})
	base := g.Revisions[0].Version
	e.stdin = "one"
	e.ok("edit", g.ID, "a.txt", "--base", base)
	e.stdin = "two"
	e.fails(1, "changed on GitHub", "edit", g.ID, "a.txt", "--base", base)
	if e.file(g.ID, "a.txt") != "one" {
		t.Errorf("got %q", e.file(g.ID, "a.txt"))
	}
}

func TestEditReportsWhatItWentOver(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a"})
	e.s.Before("PATCH /gists/", func() { e.s.Edit(g.ID, "a.txt", "theirs") })
	e.stdin = "mine"
	_, stderr := e.ok("edit", g.ID, "a.txt", "--base", g.Revisions[0].Version)
	if want := e.s.Get(g.ID).Revisions[1].Version; !strings.Contains(stderr, want) {
		t.Errorf("stderr is %q, want the revision %s", stderr, want)
	}
}

func TestEditWithABaseDescription(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("old", false, map[string]string{"a.txt": "a"})
	e.fails(1, "changed on GitHub", "edit", g.ID, "-d", "new", "--base-desc", "other")
	e.ok("edit", g.ID, "-d", "new", "--base-desc", "old")
	e.fails(2, "--base-desc goes with -d", "edit", g.ID, "--base-desc", "new")
}

func TestEditIsRefusedWhileAnotherProcessEdits(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a"})
	l, err := stateStore().Lock(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	e.stdin = "new"
	e.fails(1, "being edited in another gist process", "edit", g.ID, "a.txt")
	e.fails(1, "being edited in another gist process", "delete", g.ID, "--yes")
}

func TestEditLeavesADraftAndSaysSo(t *testing.T) {
	e := newEnv(t)
	g := e.s.Add("", false, map[string]string{"a.txt": "a"})
	l, err := stateStore().Lock(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SaveDraft(draftOf(g.ID, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	e.stdin = "new"
	_, stderr := e.ok("edit", g.ID, "a.txt")
	if !strings.Contains(stderr, "a.txt has a draft") {
		t.Errorf("stderr is %q", stderr)
	}
	if _, ok, _ := stateStore().Draft(g.ID, "a.txt"); !ok {
		t.Error("the draft is gone")
	}
}

func TestDelete(t *testing.T) {
	e := newEnv(t)
	a := e.s.Add("a", false, map[string]string{"a.txt": "a"})
	b := e.s.Add("b", false, map[string]string{"b.txt": "b"})
	e.ok("list")
	e.fails(1, "needs --yes", "delete", a.ID)
	if e.s.Get(a.ID) == nil {
		t.Fatal("deleted without --yes")
	}
	e.ok("rm", a.ID, b.ID, "--yes")
	if e.s.Get(a.ID) != nil || e.s.Get(b.ID) != nil {
		t.Error("not deleted")
	}
	if stdout, _ := e.ok("list"); stdout != "" {
		t.Errorf("still in the kept list: %q", stdout)
	}
}

func TestDeleteFindsThemAllFirst(t *testing.T) {
	e := newEnv(t)
	a := e.s.Add("a", false, map[string]string{"a.txt": "a"})
	e.fails(1, "no gist matches", "delete", a.ID, "nothing", "--yes")
	if e.s.Get(a.ID) == nil {
		t.Error("deleted one of a list with a mistake in it")
	}
}

func TestExitCodes(t *testing.T) {
	e := newEnv(t)
	e.fails(2, "accepts between 1 and 2 arg", "view")
	e.fails(2, "unknown flag", "list", "--nope")
	e.fails(2, "requires at least 1 arg", "delete")
	e.s.Fail("GET /gists", http.StatusInternalServerError)
	e.fails(1, "refused by the test", "list")
}

func TestNoToken(t *testing.T) {
	e := newEnv(t)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("PATH", "")
	e.fails(1, "gh auth login", "list")
}

// stateStore is the state directory of the test's home.
func stateStore() *state.Store { return state.Open(xdg.StateDir()) }

func draftOf(gist, file string) state.Draft {
	return state.Draft{Gist: gist, File: file, Revision: "r", Original: []byte("o"), Content: []byte("c")}
}
