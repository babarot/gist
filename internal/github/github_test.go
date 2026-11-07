package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/model"
)

var ctx = context.Background()

func client(t *testing.T) (*Client, *fakegist.Server) {
	t.Helper()
	s := fakegist.New(t)
	c, err := New(logging.Discard(), Options{Token: "test-token", Transport: s.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}

// addGists makes n gists in s.
func addGists(s *fakegist.Server, n int) {
	for i := range n {
		s.Add(fmt.Sprintf("gist %d", i), false, map[string]string{"f.txt": "x"})
	}
}

func listAll(t *testing.T, c *Client, etags []string) ([]model.ListPage, []int) {
	t.Helper()
	var got []int
	pages, err := c.List(ctx, etags, func(p model.ListPage) { got = append(got, p.Number) })
	if err != nil {
		t.Fatal(err)
	}
	return pages, got
}

func TestListReadsEveryPage(t *testing.T) {
	c, s := client(t)
	addGists(s, 65)
	pages, got := listAll(t, c, nil)
	if len(pages) != 3 {
		t.Fatalf("got %d pages", len(pages))
	}
	slices.Sort(got)
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("got was called with %v", got)
	}
	total := 0
	for i, p := range pages {
		if p.Number != i+1 || p.ETag == "" || p.NotModified {
			t.Errorf("page %d is %+v", i, p)
		}
		total += len(p.Gists)
	}
	if total != 65 {
		t.Errorf("got %d gists", total)
	}
	if !strings.HasPrefix(s.Requests()[0], "GET /gists?per_page=30&page=1") {
		t.Errorf("the first request was %s", s.Requests()[0])
	}
}

func TestListOfNothing(t *testing.T) {
	c, _ := client(t)
	pages, _ := listAll(t, c, nil)
	if len(pages) != 1 || len(pages[0].Gists) != 0 {
		t.Errorf("got %+v", pages)
	}
}

func etags(pages []model.ListPage) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.ETag
	}
	return out
}

func TestRefreshUnchanged(t *testing.T) {
	c, s := client(t)
	addGists(s, 65)
	first, _ := listAll(t, c, nil)
	again, _ := listAll(t, c, etags(first))
	// Without a Link header on a 304, the pages come from what was known
	if len(again) != 3 {
		t.Fatalf("got %d pages", len(again))
	}
	for i, p := range again {
		if !p.NotModified || len(p.Gists) != 0 {
			t.Errorf("page %d is %+v", i, p)
		}
		if p.ETag != first[i].ETag {
			t.Errorf("page %d's ETag went from %q to %q", i, first[i].ETag, p.ETag)
		}
	}
}

func TestRefreshOnlyChangedPage(t *testing.T) {
	c, s := client(t)
	addGists(s, 65)
	first, _ := listAll(t, c, nil)
	// The oldest gists are on the last page
	s.Comment(first[2].Gists[0].ID, "bob", "hi")
	again, _ := listAll(t, c, etags(first))
	for i, want := range []bool{true, true, false} {
		if again[i].NotModified != want {
			t.Errorf("page %d: NotModified is %v", i+1, again[i].NotModified)
		}
	}
}

func TestRefreshAfterANewGist(t *testing.T) {
	c, s := client(t)
	addGists(s, 60)
	first, _ := listAll(t, c, nil)
	if len(first) != 2 {
		t.Fatalf("got %d pages", len(first))
	}
	addGists(s, 1)
	again, _ := listAll(t, c, etags(first))
	if len(again) != 3 {
		t.Fatalf("a new gist spilling onto a new page: got %d pages", len(again))
	}
}

func TestRefreshDropsEmptiedPages(t *testing.T) {
	c, s := client(t)
	addGists(s, 31)
	first, _ := listAll(t, c, nil)
	s.Remove(first[1].Gists[0].ID)
	again, _ := listAll(t, c, etags(first))
	if len(again) != 1 || !again[0].NotModified {
		t.Errorf("got %+v", again)
	}
}

func TestListFailsOnFirstPage(t *testing.T) {
	c, s := client(t)
	s.Fail("GET /gists", http.StatusInternalServerError)
	if _, err := c.List(ctx, nil, func(model.ListPage) {}); err == nil {
		t.Fatal("no error")
	}
}

// failLater fails every request but that of the first page.
type failLater struct{ next http.RoundTripper }

func (f failLater) RoundTrip(r *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(r.URL.RawQuery, "&page=1") {
		return nil, errors.New("the network went away")
	}
	return f.next.RoundTrip(r)
}

func TestListFailsOnALaterPage(t *testing.T) {
	s := fakegist.New(t)
	addGists(s, 200)
	c, err := New(logging.Discard(), Options{Token: "t", Transport: failLater{s.Transport()}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.List(ctx, nil, func(model.ListPage) {})
	if err == nil || !strings.Contains(err.Error(), "the network went away") {
		t.Errorf("got %v", err)
	}
}

func TestGist(t *testing.T) {
	c, s := client(t)
	g := s.Add("two files", false, map[string]string{"b.txt": "b", "a.txt": "a"})
	got, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != g.ID || got.Description != "two files" || got.Public || got.Owner != "alice" {
		t.Errorf("got %+v", got)
	}
	if len(got.Files) != 2 || got.Files[0].Name != "a.txt" || got.Files[1].Content != "b" {
		t.Errorf("files are %+v", got.Files)
	}
	if got.Revision() != g.Revisions[0].Version {
		t.Errorf("revision is %q", got.Revision())
	}
}

func TestListHasNoContentOrHistory(t *testing.T) {
	c, s := client(t)
	s.Add("d", true, map[string]string{"a.txt": "a"})
	pages, _ := listAll(t, c, nil)
	g := pages[0].Gists[0]
	if !g.Public || g.Files[0].Content != "" || g.Files[0].RawURL == "" || g.Revision() != "" {
		t.Errorf("got %+v", g)
	}
}

// raw is a file go-gh's sanitizing would change: the JSON content of a gist
// has its control characters written as carets.
const raw = "col\tumn\r\n\x1b[31mred\x1b[0m\n❤️\n"

func TestRawIsTheFileAsWritten(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": raw})
	got, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Files[0].Content == raw {
		t.Error("the API client no longer rewrites the content: the design's reason to read raw_url is gone")
	}
	b, err := c.Raw(ctx, got.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	if b != raw {
		t.Errorf("got %q, want %q", b, raw)
	}
}

func TestRawOfAnOldRevision(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "one"})
	old, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.Edit(g.ID, "a.txt", "two")
	if b, err := c.Raw(ctx, old.Files[0]); err != nil || b != "one" {
		t.Errorf("got %q, %v", b, err)
	}
}

func TestRawOfAFileWithASpaceInItsName(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a b.txt": "x"})
	got, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := c.Raw(ctx, got.Files[0]); err != nil || b != "x" {
		t.Errorf("got %q, %v", b, err)
	}
}

func TestLargeFile(t *testing.T) {
	c, s := client(t)
	content := strings.Repeat("x", fakegist.MaxContent+10)
	g := s.Add("", false, map[string]string{"big.txt": content})
	got, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	f := got.Files[0]
	if !f.Truncated || len(f.Content) != fakegist.MaxContent {
		t.Errorf("truncated %v, %d bytes", f.Truncated, len(f.Content))
	}
	if b, err := c.Raw(ctx, f); err != nil || b != content {
		t.Errorf("got %d bytes, %v", len(b), err)
	}
}

func TestRawRefuses(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "abc"})
	got, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	wrong := got.Files[0]
	wrong.Size = 2
	if _, err := c.Raw(ctx, wrong); !errors.Is(err, ErrRawIncomplete) {
		t.Errorf("fewer bytes than read: got %v", err)
	}
	large := got.Files[0]
	large.Size = MaxRaw + 1
	if _, err := c.Raw(ctx, large); !errors.Is(err, ErrRawTooLarge) {
		t.Errorf("too large: got %v", err)
	}
	gone := got.Files[0]
	s.Remove(g.ID)
	if _, err := c.Raw(ctx, gone); err == nil {
		t.Error("no error for a gist that is gone")
	}
}

func TestCreate(t *testing.T) {
	c, s := client(t)
	got, err := c.Create(ctx, model.NewGist{Description: "d", Files: map[string]string{"a.txt": raw}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Public || got.Description != "d" {
		t.Errorf("got %+v", got)
	}
	if s.Get(got.ID).Revisions[0].Files["a.txt"] != raw {
		t.Error("the content was changed on the way")
	}
}

func TestUpdate(t *testing.T) {
	c, s := client(t)
	str := func(s string) *string { return &s }
	g := s.Add("old", false, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
	first := g.Revisions[0].Version

	got, err := c.Update(ctx, g.ID, model.Change{Description: str("new")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "new" || got.Revision() != first {
		t.Errorf("a change of description alone: %+v", got)
	}

	got, err = c.Update(ctx, g.ID, model.Change{Files: map[string]model.FileChange{
		"a.txt": {Content: str(raw)},
		"b.txt": {Remove: true},
		"c.txt": {Name: str("d.txt")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 2 || got.History[1] != first {
		t.Errorf("history is %v", got.History)
	}
	files := s.Get(g.ID).Revisions[0].Files
	if files["a.txt"] != raw || len(files) != 2 || files["d.txt"] != "c" {
		t.Errorf("files are %q", files)
	}
}

func TestUpdateAddsAFile(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	content := "b"
	if _, err := c.Update(ctx, g.ID, model.Change{Files: map[string]model.FileChange{"b.txt": {Content: &content}}}); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(g.ID).Revisions[0].Files; len(got) != 2 || got["b.txt"] != "b" {
		t.Errorf("files are %q", got)
	}
}

func TestInvalidUTF8IsNotSent(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	bad := "\xff"
	_, err := c.Update(ctx, g.ID, model.Change{Files: map[string]model.FileChange{"a.txt": {Content: &bad}}})
	if !errors.Is(err, errInvalidUTF8) {
		t.Errorf("update: got %v", err)
	}
	_, err = c.Create(ctx, model.NewGist{Files: map[string]string{"a.txt": bad}})
	if !errors.Is(err, errInvalidUTF8) {
		t.Errorf("create: got %v", err)
	}
	for _, r := range s.Requests() {
		if !strings.HasPrefix(r, "GET") {
			t.Errorf("sent %s", r)
		}
	}
}

func TestEmptyContentIsNotSent(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	empty := ""
	_, err := c.Update(ctx, g.ID, model.Change{Files: map[string]model.FileChange{"a.txt": {Content: &empty}}})
	if !errors.Is(err, ErrEmptyContent) {
		t.Errorf("got %v", err)
	}
	if _, ok := s.Get(g.ID).Revisions[0].Files["a.txt"]; !ok {
		t.Error("the file is gone")
	}
}

func TestDelete(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	if err := c.Delete(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if s.Get(g.ID) != nil {
		t.Error("the gist is still there")
	}
	if err := c.Delete(ctx, g.ID); !IsNotFound(err) {
		t.Errorf("deleting it twice: %v", err)
	}
}

func TestStarred(t *testing.T) {
	c, s := client(t)
	s.Add("mine", false, map[string]string{"a.txt": "a"})
	for i := range 120 {
		s.Star("bob", fmt.Sprint(i), map[string]string{"a.txt": "a"})
	}
	got, err := c.Starred(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 120 || got[0].Owner != "bob" {
		t.Errorf("got %d gists, the first %+v", len(got), got[0])
	}
	pages, _ := listAll(t, c, nil)
	if n := len(pages[0].Gists); n != 1 {
		t.Errorf("the user's list has %d gists", n)
	}
}

func TestComments(t *testing.T) {
	c, s := client(t)
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	for i := range 120 {
		s.Comment(g.ID, "bob", fmt.Sprint(i))
	}
	got, err := c.Comments(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 120 || got[0].Body != "0" || got[119].Body != "119" || got[0].Author != "bob" {
		t.Errorf("got %d comments, the first %+v", len(got), got[0])
	}
}

func TestRefusalsNameTheFineGrainedPermission(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		c, s := client(t)
		g := s.Add("", false, map[string]string{"a.txt": "a"})
		s.Fail("GET /gists/"+g.ID, status)
		_, err := c.Get(ctx, g.ID)
		if err == nil || !strings.Contains(err.Error(), "Gists read and write") {
			t.Errorf("%d: got %v", status, err)
		}
	}
}

func TestCheckAccess(t *testing.T) {
	scopes := func(s string) *string { return &s }
	for _, tt := range []struct {
		name   string
		scopes *string
		fail   int
		want   Access
		err    bool
	}{
		{"classic with gist", scopes("repo, gist"), 0, Access{Login: "alice", Scoped: true, GistScope: true}, false},
		{"classic without gist", scopes("repo"), 0, Access{Login: "alice", Scoped: true}, false},
		{"classic with no scopes", scopes(""), 0, Access{Login: "alice", Scoped: true}, false},
		{"fine-grained", nil, 0, Access{Login: "alice"}, false},
		{"fine-grained without the permission", nil, http.StatusForbidden, Access{Login: "alice"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, s := client(t)
			s.Scopes = tt.scopes
			if tt.fail != 0 {
				s.Fail("GET /gists", tt.fail)
			}
			got, err := c.CheckAccess(ctx)
			if got != tt.want || (err != nil) != tt.err {
				t.Errorf("got %+v, %v", got, err)
			}
		})
	}
}

func TestSource(t *testing.T) {
	for in, want := range map[string]string{
		"GH_TOKEN": "GH_TOKEN", "GITHUB_TOKEN": "GITHUB_TOKEN", "oauth_token": "gh", "gh": "gh",
	} {
		if got := Source(in); got != want {
			t.Errorf("%s: got %q", in, got)
		}
	}
}

func TestLogHoldsNoBodies(t *testing.T) {
	s := fakegist.New(t)
	var buf bytes.Buffer
	c, err := New(logging.New(&buf, slog.LevelDebug), Options{Token: "secret-token", Transport: s.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Add("the description", false, map[string]string{"a.txt": "the content"})
	got, err := c.Get(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Raw(ctx, got.Files[0]); err != nil {
		t.Fatal(err)
	}
	content := "the new content"
	if _, err := c.Update(ctx, g.ID, model.Change{Files: map[string]model.FileChange{"a.txt": {Content: &content}}}); err != nil {
		t.Fatal(err)
	}
	log := buf.String()
	if !strings.Contains(log, g.ID) {
		t.Errorf("the log does not name the gist: %s", log)
	}
	for _, secret := range []string{"secret-token", "the description", "the content", "the new content"} {
		if strings.Contains(log, secret) {
			t.Errorf("the log holds %q: %s", secret, log)
		}
	}
}

func TestReleases(t *testing.T) {
	c, srv := client(t)
	at := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	srv.Releases = []fakegist.Release{
		{TagName: "v2.1.0", Draft: true},
		{TagName: "v2.1.0-rc.1", Prerelease: true},
		{TagName: "v2.0.0", PublishedAt: at, Body: "notes"},
	}
	got, err := c.Releases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != "2.0.0" || got[0].Body != "notes" || !got[0].Published.Equal(at) {
		t.Errorf("got %+v", got)
	}
}
