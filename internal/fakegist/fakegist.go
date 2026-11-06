// Package fakegist is a made-up GitHub Gist API for tests: an httptest server
// a client reaches through Transport, whatever host it asks for.
//
// The state is kept in the shapes of the API and answers are made from it, so
// the client's reading is not copied here. A call it does not know fails the
// test. GitHub's behaviors that gist depends on are reproduced, as seen on
// the real API: pages with Link headers, weak ETags, a 304 with neither Link
// nor the weak form of the ETag, raw_url served by URL alone, file content cut
// at 1 MB, a change of description alone making no revision, a PATCH of the same
// content making one, empty content removing a file, and a rename onto a taken
// name doing nothing. What it cannot catch is a wrong assumption about GitHub
// shared by gist and this fake.
package fakegist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// MaxContent is how much of a file's content GET /gists/{id} gives.
const MaxContent = 1 << 20

// Epoch is when the server's clock starts. It moves a minute at each change.
var Epoch = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

const (
	apiHost = "api.github.com"
	rawHost = "gist.githubusercontent.com"
)

// Gist is a gist of the server.
type Gist struct {
	ID          string
	Description string
	Owner       string
	Public      bool
	Created     time.Time
	Updated     time.Time
	Comments    []Comment
	// Revisions are the file contents at each revision, newest first.
	Revisions []Revision
}

// Revision is a state of a gist's files.
type Revision struct {
	Version string
	Files   map[string]string
}

// Comment is a comment on a gist.
type Comment struct {
	ID   int64
	User string
	Body string
	At   time.Time
}

func (g *Gist) files() map[string]string { return g.Revisions[0].Files }

// Server is the made-up GitHub. Its fields may be set before the first
// request.
type Server struct {
	// Login is who the token belongs to.
	Login string
	// Scopes is the X-OAuth-Scopes header, not sent when nil, as for a
	// fine-grained token.
	Scopes *string

	t        testing.TB
	srv      *httptest.Server
	mu       sync.Mutex
	gists    []*Gist // newest created first, the order GitHub lists them in
	starred  []string
	now      time.Time
	next     int
	fail     map[string]int
	requests []string
	before   []hook
}

type hook struct {
	prefix string
	f      func()
}

// New starts a server for the length of the test.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		Login: "alice",
		t:     t,
		now:   Epoch,
		fail:  map[string]int{},
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.srv.Close)
	return s
}

// Transport sends every request to the server, keeping the host it was for in
// a header.
func (s *Server) Transport() http.RoundTripper { return rewrite{s.srv.URL} }

type rewrite struct{ to string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	to, err := url.Parse(r.to)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.Header.Set("X-Fake-Host", req.URL.Host)
	req.URL.Scheme, req.URL.Host = to.Scheme, to.Host
	return http.DefaultTransport.RoundTrip(req)
}

// Add makes a gist with files and returns it, newest in the list.
func (s *Server) Add(description string, public bool, files map[string]string) *Gist {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.add(s.Login, description, public, files)
}

func (s *Server) add(owner, description string, public bool, files map[string]string) *Gist {
	g := &Gist{
		ID:          s.id(),
		Description: description,
		Owner:       owner,
		Public:      public,
		Created:     s.tick(),
		Updated:     s.now,
		Revisions:   []Revision{{Version: s.id(), Files: maps.Clone(files)}},
	}
	s.gists = append([]*Gist{g}, s.gists...)
	return g
}

// Star adds a gist of another user to the user's starred gists.
func (s *Server) Star(owner, description string, files map[string]string) *Gist {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.add(owner, description, true, files)
	s.gists = s.gists[1:]
	s.gists = append(s.gists, g) // reachable by ID, not in the user's list
	s.starred = append(s.starred, g.ID)
	return g
}

// Remove deletes a gist, as from elsewhere.
func (s *Server) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gists = slices.DeleteFunc(s.gists, func(g *Gist) bool { return g.ID == id })
}

// Comment adds a comment on a gist.
func (s *Server) Comment(id, user, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.find(id)
	g.Comments = append(g.Comments, Comment{ID: int64(len(g.Comments) + 1), User: user, Body: body, At: s.tick()})
}

// Edit changes a file as from elsewhere, making a revision.
func (s *Server) Edit(id, name, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.find(id)
	files := maps.Clone(g.files())
	files[name] = content
	s.commit(g, files)
}

// SetDescription changes the description as from elsewhere.
func (s *Server) SetDescription(id, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.find(id)
	g.Description = description
	g.Updated = s.tick()
}

// Before calls f before answering each request whose "METHOD path" starts
// with prefix, without the server's lock, so f may change the state: the moment
// between one request of the client and its next.
func (s *Server) Before(prefix string, f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.before = append(s.before, hook{prefix, f})
}

// Fail has requests whose "METHOD path" starts with prefix answered with
// status.
func (s *Server) Fail(prefix string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail[prefix] = status
}

// Requests are the "METHOD path?query" of each request so far.
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

// Get is the gist as it is now.
func (s *Server) Get(id string) *Gist {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.find(id)
}

func (s *Server) tick() time.Time {
	s.now = s.now.Add(time.Minute)
	return s.now
}

func (s *Server) id() string {
	s.next++
	h := sha256.Sum256([]byte(strconv.Itoa(s.next)))
	return hex.EncodeToString(h[:])[:32]
}

func (s *Server) find(id string) *Gist {
	for _, g := range s.gists {
		if g.ID == id {
			return g
		}
	}
	return nil
}

func (s *Server) commit(g *Gist, files map[string]string) {
	g.Revisions = append([]Revision{{Version: s.id(), Files: files}}, g.Revisions...)
	g.Updated = s.tick()
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	route := r.Method + " " + r.URL.Path
	s.mu.Lock()
	hooks := slices.Clone(s.before)
	s.mu.Unlock()
	for _, h := range hooks {
		if strings.HasPrefix(route, h.prefix) {
			h.f()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.Method+" "+r.URL.RequestURI())
	for prefix, status := range s.fail {
		if strings.HasPrefix(route, prefix) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"message":"refused by the test (%d)"}`, status)
			return
		}
	}
	if r.Header.Get("X-Fake-Host") == rawHost {
		s.serveRaw(w, r)
		return
	}
	if host := r.Header.Get("X-Fake-Host"); host != apiHost {
		s.t.Errorf("fakegist: a request to %s: %s", host, route)
		http.Error(w, "unknown host", http.StatusNotImplemented)
		return
	}
	if r.Header.Get("Authorization") == "" {
		s.message(w, http.StatusUnauthorized, "Requires authentication")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case route == "GET /user":
		if s.Scopes != nil {
			w.Header().Set("X-OAuth-Scopes", *s.Scopes)
		}
		s.json(w, r, http.StatusOK, map[string]any{"login": s.Login})
	case route == "GET /gists":
		paged(s, w, r, s.listed(), s.summaryOf, 30)
	case route == "POST /gists":
		s.create(w, r)
	case route == "GET /gists/starred":
		var gists []*Gist
		for _, id := range s.starred {
			gists = append(gists, s.find(id))
		}
		paged(s, w, r, gists, s.summaryOf, 30)
	case len(parts) == 2 && parts[0] == "gists":
		s.gist(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "gists" && parts[2] == "comments" && r.Method == http.MethodGet:
		g := s.find(parts[1])
		if g == nil {
			s.message(w, http.StatusNotFound, "Not Found")
			return
		}
		paged(s, w, r, g.Comments, commentJSON, 30)
	default:
		s.t.Errorf("fakegist: an unknown call: %s", route)
		http.Error(w, "unknown call", http.StatusNotImplemented)
	}
}

// listed are the gists of the user's list.
func (s *Server) listed() []*Gist {
	var gists []*Gist
	for _, g := range s.gists {
		if g.Owner == s.Login {
			gists = append(gists, g)
		}
	}
	return gists
}

func (s *Server) gist(w http.ResponseWriter, r *http.Request, id string) {
	g := s.find(id)
	if g == nil {
		s.message(w, http.StatusNotFound, "Not Found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.json(w, r, http.StatusOK, s.full(g))
	case http.MethodPatch:
		s.update(w, r, g)
	case http.MethodDelete:
		s.gists = slices.DeleteFunc(s.gists, func(o *Gist) bool { return o == g })
		w.WriteHeader(http.StatusNoContent)
	default:
		s.t.Errorf("fakegist: an unknown call: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unknown call", http.StatusNotImplemented)
	}
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
		Public      bool   `json:"public"`
		Files       map[string]struct {
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.message(w, http.StatusBadRequest, "Problems parsing JSON")
		return
	}
	files := map[string]string{}
	for name, f := range body.Files {
		if f.Content == "" {
			s.message(w, http.StatusUnprocessableEntity, "Validation Failed")
			return
		}
		files[name] = f.Content
	}
	if len(files) == 0 {
		s.message(w, http.StatusUnprocessableEntity, "Validation Failed")
		return
	}
	s.json(w, r, http.StatusCreated, s.full(s.add(s.Login, body.Description, body.Public, files)))
}

func (s *Server) update(w http.ResponseWriter, r *http.Request, g *Gist) {
	var body struct {
		Description *string `json:"description"`
		Files       map[string]*struct {
			Content  *string `json:"content"`
			Filename *string `json:"filename"`
		} `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.message(w, http.StatusBadRequest, "Problems parsing JSON")
		return
	}
	files := maps.Clone(g.files())
	for name, f := range body.Files {
		content, ok := files[name]
		// A name not in the gist adds a file, which takes content
		if !ok && (f == nil || f.Content == nil) {
			s.message(w, http.StatusUnprocessableEntity, "Validation Failed")
			return
		}
		delete(files, name)
		if f == nil {
			continue
		}
		if f.Content != nil {
			content = *f.Content
		}
		// Empty content removes the file, and a name already taken leaves it
		// under the name it had
		if content == "" {
			continue
		}
		if f.Filename != nil {
			if _, taken := g.files()[*f.Filename]; !taken {
				name = *f.Filename
			}
		}
		files[name] = content
	}
	// Any change of files makes a revision, even one that changes nothing
	if len(body.Files) > 0 {
		s.commit(g, files)
	}
	if body.Description != nil && *body.Description != g.Description {
		g.Description = *body.Description
		g.Updated = s.tick()
	}
	s.json(w, r, http.StatusOK, s.full(g))
}

// serveRaw answers a raw_url, /<owner>/<id>/raw/<version>/<file>.
func (s *Server) serveRaw(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "" {
		s.t.Errorf("fakegist: the token was sent to %s", rawHost)
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[2] != "raw" {
		s.t.Errorf("fakegist: an unknown raw path: %s", r.URL.Path)
		http.Error(w, "unknown call", http.StatusNotImplemented)
		return
	}
	if g := s.find(parts[1]); g != nil {
		name, err := url.PathUnescape(parts[4])
		if err == nil {
			for _, rev := range g.Revisions {
				if content, ok := rev.Files[name]; ok && rev.Version == parts[3] {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					fmt.Fprint(w, content)
					return
				}
			}
		}
	}
	http.NotFound(w, r)
}

func (s *Server) message(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

// json answers with v and a weak ETag, or with a 304 for the ETag asked for.
func (s *Server) json(w http.ResponseWriter, r *http.Request, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		s.t.Errorf("fakegist: %v", err)
		return
	}
	sum := sha256.Sum256(b)
	tag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("X-RateLimit-Remaining", "4999")
	if strings.TrimPrefix(r.Header.Get("If-None-Match"), "W/") == tag {
		w.Header().Del("Link")
		w.Header().Set("ETag", tag)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", "W/"+tag)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// paged answers with the page of items that per_page and page ask for, and Link.
func paged[T any](s *Server, w http.ResponseWriter, r *http.Request, items []T, each func(T) any, perPage int) {
	q := r.URL.Query()
	if n, err := strconv.Atoi(q.Get("per_page")); err == nil {
		perPage = n
	}
	page := 1
	if n, err := strconv.Atoi(q.Get("page")); err == nil {
		page = n
	}
	last := max(1, (len(items)+perPage-1)/perPage)
	link := func(n int, rel string) string {
		v := r.URL.Query()
		v.Set("per_page", strconv.Itoa(perPage))
		v.Set("page", strconv.Itoa(n))
		return fmt.Sprintf(`<https://%s%s?%s>; rel="%s"`, apiHost, r.URL.Path, v.Encode(), rel)
	}
	var links []string
	if page > 1 {
		links = append(links, link(page-1, "prev"), link(1, "first"))
	}
	if page < last {
		links = append(links, link(page+1, "next"), link(last, "last"))
	}
	if len(links) > 0 {
		w.Header().Set("Link", strings.Join(links, ", "))
	}
	out := []any{}
	for i := (page - 1) * perPage; i < page*perPage && i < len(items); i++ {
		out = append(out, each(items[i]))
	}
	s.json(w, r, http.StatusOK, out)
}

func commentJSON(c Comment) any {
	return map[string]any{
		"id":         c.ID,
		"user":       map[string]any{"login": c.User},
		"body":       c.Body,
		"created_at": c.At.Format(time.RFC3339),
		"updated_at": c.At.Format(time.RFC3339),
	}
}

func (s *Server) rawURL(g *Gist, version, name string) string {
	return fmt.Sprintf("https://%s/%s/%s/raw/%s/%s", rawHost, g.Owner, g.ID, version, url.PathEscape(name))
}

func (s *Server) summaryOf(g *Gist) any { return s.summary(g) }

// summary is a gist as in a list: its files without content.
func (s *Server) summary(g *Gist) map[string]any {
	files := map[string]any{}
	for name, f := range s.fileFields(g) {
		files[name] = f
	}
	return s.fields(g, files)
}

// full is a gist as read alone: with content, cut at MaxContent, and history.
func (s *Server) full(g *Gist) map[string]any {
	files := map[string]any{}
	for name, f := range s.fileFields(g) {
		content := g.files()[name]
		f["truncated"] = len(content) > MaxContent
		if len(content) > MaxContent {
			content = content[:MaxContent]
		}
		f["content"] = content
		files[name] = f
	}
	m := s.fields(g, files)
	history := make([]any, len(g.Revisions))
	for i, r := range g.Revisions {
		history[i] = map[string]any{"version": r.Version}
	}
	m["history"] = history
	return m
}

func (s *Server) fileFields(g *Gist) map[string]map[string]any {
	files := map[string]map[string]any{}
	for name, content := range g.files() {
		files[name] = map[string]any{
			"filename": name,
			"type":     "text/plain",
			"language": nil,
			"size":     len(content),
			"raw_url":  s.rawURL(g, g.Revisions[0].Version, name),
		}
	}
	return files
}

func (s *Server) fields(g *Gist, files map[string]any) map[string]any {
	return map[string]any{
		"id":          g.ID,
		"description": g.Description,
		"owner":       map[string]any{"login": g.Owner},
		"public":      g.Public,
		"html_url":    "https://gist.github.com/" + g.Owner + "/" + g.ID,
		"comments":    len(g.Comments),
		"created_at":  g.Created.Format(time.RFC3339),
		"updated_at":  g.Updated.Format(time.RFC3339),
		"files":       files,
	}
}
