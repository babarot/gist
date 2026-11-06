// Package github is gist's only way to GitHub: it reads and writes gists
// through the REST API, with the token go-gh finds. It returns model types.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/browser"

	"github.com/babarot/gist/v2/internal/model"
)

const (
	// Host is the GitHub gist talks to.
	Host = "github.com"

	// listPerPage is the same for a first read and a refresh, as an ETag
	// is only good for the page boundaries it was made with.
	listPerPage = 30
	// otherPerPage is the most a page holds, for lists read one page at a
	// time.
	otherPerPage = 100
	// listConcurrency is how many pages of the list are read at once.
	listConcurrency = 4
	// timeout is how long a request may take.
	timeout = 30 * time.Second
	// MaxRaw is the largest file read whole: past this size GitHub serves
	// a file's whole content only through git.
	MaxRaw = 10 << 20
)

var (
	// ErrRawTooLarge says a file is over MaxRaw.
	ErrRawTooLarge = errors.New("the file is over 10 MB; use gist clone")
	// ErrRawIncomplete says fewer or more bytes came than the file's size.
	ErrRawIncomplete = errors.New("the whole file could not be read; use gist clone")
)

// ErrEmptyContent says a file would be left empty, which GitHub takes as
// removing the file. A file is only ever removed by asking for that.
var ErrEmptyContent = errors.New("GitHub removes an empty file: remove it on purpose instead")

// Client talks to GitHub.
type Client struct {
	rest *api.RESTClient
	// raw reads raw_url, on a host that serves secret gists by their URL
	// alone, so the token is not sent there.
	raw *http.Client
	log *slog.Logger
}

// Options configure a Client.
type Options struct {
	// Token is the token to use.
	Token string
	// Transport is for tests; nil uses the network.
	Transport http.RoundTripper
}

// New returns a client.
func New(log *slog.Logger, o Options) (*Client, error) {
	next := o.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	t := transport{next: next, log: log}
	// LogIgnoreEnv: GH_DEBUG=api would make go-gh print request and response
	// bodies to stderr, gists among them and over the TUI. Requests are
	// logged by transport instead, without bodies.
	rest, err := api.NewRESTClient(api.ClientOptions{Host: Host, AuthToken: o.Token, Transport: t,
		Timeout: timeout, LogIgnoreEnv: true})
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest, raw: &http.Client{Transport: t, Timeout: timeout}, log: log}, nil
}

// Viewer is who the token belongs to.
func (c *Client) Viewer(ctx context.Context) (string, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := c.rest.DoWithContext(ctx, http.MethodGet, "user", nil, &user); err != nil {
		return "", explain(err)
	}
	return user.Login, nil
}

// Browse opens url in the browser gh uses.
func Browse(url string, stdout, stderr io.Writer) error {
	return browser.New("", stdout, stderr).Browse(url)
}

// Token finds the token to use and where it came from, which Source names.
// The token is "" when there is none.
func Token() (token, source string) { return auth.TokenForHost(Host) }

// Source is where a token came from, as gist says it.
func Source(source string) string {
	if source == "GH_TOKEN" || source == "GITHUB_TOKEN" {
		return source
	}
	return "gh"
}

// NoToken says what to do when Token found none.
const NoToken = `no GitHub token found. Looked in GH_TOKEN, GITHUB_TOKEN, gh's hosts.yml and gh auth token.
Run gh auth login, or set GITHUB_TOKEN.
A fine-grained token needs the Gists read and write permission`

// tokenHint is added to the errors a token without access to gists gives.
const tokenHint = "a fine-grained token needs the Gists read and write permission"

// explain adds tokenHint to a refusal, which a token lacking the permission
// is one cause of.
func explain(err error) error {
	if s := status(err); s == http.StatusForbidden || s == http.StatusNotFound {
		return fmt.Errorf("%w (%s)", err, tokenHint)
	}
	return err
}

// status is the HTTP status of err, or 0.
func status(err error) int {
	if e, ok := errors.AsType[*api.HTTPError](err); ok {
		return e.StatusCode
	}
	return 0
}

// IsNotFound says err is GitHub answering that a gist does not exist.
func IsNotFound(err error) bool { return status(err) == http.StatusNotFound }

// Access is what a token can do.
type Access struct {
	Login string
	// Scoped says GitHub named the token's scopes, as it does for a classic
	// token and not for a fine-grained one.
	Scoped bool
	// GistScope says a Scoped token has the gist scope.
	GistScope bool
}

// CheckAccess asks GitHub who the token belongs to and what it may do.
func (c *Client) CheckAccess(ctx context.Context) (Access, error) {
	resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, "user", nil)
	if err != nil {
		return Access{}, err
	}
	defer resp.Body.Close()
	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return Access{}, err
	}
	a := Access{Login: user.Login}
	if scopes, ok := resp.Header[http.CanonicalHeaderKey("X-OAuth-Scopes")]; ok {
		a.Scoped = true
		for s := range strings.SplitSeq(strings.Join(scopes, ","), ",") {
			a.GistScope = a.GistScope || strings.TrimSpace(s) == "gist"
		}
		return a, nil
	}
	// A fine-grained token says nothing of what it may do: only reading
	// tells, and writing cannot be tried
	if _, err := c.listPage(ctx, 1, ""); err != nil {
		return a, explain(err)
	}
	return a, nil
}

// List reads every page of the user's gists. etags are those of the pages as
// last read, the first page's first, each asking for its page only if it
// changed. got is called with each page as it comes, in no order, never twice
// at once; it must not block, as the pages being read wait for it. What List
// returns is in order.
func (c *Client) List(ctx context.Context, etags []string, got func(model.ListPage)) ([]model.ListPage, error) {
	etag := func(n int) string {
		if n <= len(etags) {
			return etags[n-1]
		}
		return ""
	}
	first, err := c.listPage(ctx, 1, etag(1))
	if err != nil {
		return nil, explain(err)
	}
	got(first.ListPage)
	last := first.last
	if first.NotModified {
		// A 304 has no Link header. An unchanged first page means no gist
		// was made, so there are no more pages than last time
		last = len(etags)
	}
	pages := make([]model.ListPage, last)
	pages[0] = first.ListPage

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, listConcurrency)
	for n := 2; n <= last; n++ {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			p, err := c.listPage(ctx, n, etag(n))
			if err != nil {
				cancel(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			pages[n-1] = p.ListPage
			got(p.ListPage)
		})
	}
	wg.Wait()
	if err := context.Cause(ctx); err != nil {
		return nil, explain(err)
	}
	// Gists deleted since the last read leave pages with nothing on them
	for len(pages) > 1 && !pages[len(pages)-1].NotModified && len(pages[len(pages)-1].Gists) == 0 {
		pages = pages[:len(pages)-1]
	}
	return pages, nil
}

// readPage is a page of the list and the number of the last page, which a
// page says only when it is 200 and the first.
type readPage struct {
	model.ListPage
	last int
}

func (c *Client) listPage(ctx context.Context, n int, etag string) (readPage, error) {
	path := fmt.Sprintf("gists?per_page=%d&page=%d", listPerPage, n)
	resp, err := c.rest.RequestWithContext(withETag(ctx, etag), http.MethodGet, path, nil)
	if status(err) == http.StatusNotModified {
		// A 304's ETag is written differently from the 200's. The one
		// that has just worked is kept
		return readPage{ListPage: model.ListPage{Number: n, ETag: etag, NotModified: true}, last: n}, nil
	}
	if err != nil {
		return readPage{}, err
	}
	defer resp.Body.Close()
	var body []gist
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return readPage{}, fmt.Errorf("reading page %d of the list: %w", n, err)
	}
	gists := make([]model.Gist, len(body))
	for i, g := range body {
		gists[i] = g.model()
	}
	return readPage{
		ListPage: model.ListPage{Number: n, ETag: resp.Header.Get("ETag"), Gists: gists},
		last:     max(n, lastPage(resp.Header.Get("Link"))),
	}, nil
}

var linkRel = regexp.MustCompile(`<([^>]*)>;\s*rel="(\w+)"`)

// linkTo is the URL a Link header gives for rel, or "".
func linkTo(link, rel string) string {
	for _, m := range linkRel.FindAllStringSubmatch(link, -1) {
		if m[2] == rel {
			return m[1]
		}
	}
	return ""
}

// lastPage is the number of the last page a Link header names, or 0.
func lastPage(link string) int {
	u, err := url.Parse(linkTo(link, "last"))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(u.Query().Get("page"))
	return n
}

// Starred reads the gists the user starred.
func (c *Client) Starred(ctx context.Context) ([]model.Gist, error) {
	var gists []model.Gist
	err := each(c, ctx, "gists/starred", func(g gist) { gists = append(gists, g.model()) })
	return gists, err
}

// Comments reads a gist's comments, oldest first.
func (c *Client) Comments(ctx context.Context, id string) ([]model.Comment, error) {
	var comments []model.Comment
	err := each(c, ctx, "gists/"+id+"/comments", func(g comment) { comments = append(comments, g.model()) })
	return comments, err
}

// each calls f with every item of the list at path, a page at a time.
func each[T any](c *Client, ctx context.Context, path string, f func(T)) error {
	path = fmt.Sprintf("%s?per_page=%d", path, otherPerPage)
	for path != "" {
		resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, path, nil)
		if err != nil {
			return explain(err)
		}
		var items []T
		err = json.NewDecoder(resp.Body).Decode(&items)
		next := linkTo(resp.Header.Get("Link"), "next")
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		for _, item := range items {
			f(item)
		}
		path = next
	}
	return nil
}

// Get reads a gist with its revisions and the content of its files for
// showing, which is not the content to edit: see Raw.
func (c *Client) Get(ctx context.Context, id string) (model.Gist, error) {
	var g gist
	if err := c.rest.DoWithContext(ctx, http.MethodGet, "gists/"+id, nil, &g); err != nil {
		return model.Gist{}, explain(err)
	}
	return g.model(), nil
}

// Raw reads a file as it was written, at the revision f was read at. It
// fails rather than return part of a file.
func (c *Client) Raw(ctx context.Context, f model.File) (string, error) {
	if f.Size > MaxRaw {
		return "", ErrRawTooLarge
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.RawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.raw.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("reading %s: %s", f.Name, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxRaw+1))
	if err != nil {
		return "", err
	}
	if int64(len(b)) != f.Size {
		return "", ErrRawIncomplete
	}
	return string(b), nil
}

// Create makes a gist.
func (c *Client) Create(ctx context.Context, n model.NewGist) (model.Gist, error) {
	files := make(map[string]fileChange, len(n.Files))
	for name, content := range n.Files {
		if err := checkUTF8(&content); err != nil {
			return model.Gist{}, err
		}
		files[name] = fileChange{Content: &content}
	}
	body := struct {
		Description string                `json:"description"`
		Public      bool                  `json:"public"`
		Files       map[string]fileChange `json:"files"`
	}{n.Description, n.Public, files}
	return c.send(ctx, http.MethodPost, "gists", body, slog.Int("files", len(files)))
}

// Update changes a gist and returns it as it is after, History included.
func (c *Client) Update(ctx context.Context, id string, ch model.Change) (model.Gist, error) {
	files := make(map[string]*fileChange, len(ch.Files))
	for name, f := range ch.Files {
		if err := checkUTF8(f.Content, f.Name); err != nil {
			return model.Gist{}, err
		}
		if f.Remove {
			files[name] = nil // null removes the file
			continue
		}
		if f.Content != nil && *f.Content == "" {
			return model.Gist{}, ErrEmptyContent
		}
		files[name] = &fileChange{Content: f.Content, Filename: f.Name}
	}
	body := struct {
		Description *string                `json:"description,omitempty"`
		Files       map[string]*fileChange `json:"files,omitempty"`
	}{ch.Description, files}
	return c.send(ctx, http.MethodPatch, "gists/"+id, body, slog.String("gist", id), slog.Int("files", len(files)))
}

// Delete removes a gist.
func (c *Client) Delete(ctx context.Context, id string) error {
	err := c.rest.DoWithContext(ctx, http.MethodDelete, "gists/"+id, nil, nil)
	c.log.Info("delete gist", "gist", id, "err", errString(err))
	return explain(err)
}

// send writes body and reads the gist that comes back.
func (c *Client) send(ctx context.Context, method, path string, body any, attrs ...any) (model.Gist, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return model.Gist{}, err
	}
	var g gist
	err = c.rest.DoWithContext(ctx, method, path, bytes.NewReader(b), &g)
	c.log.Info("write gist", append(attrs, "method", method, "err", errString(err))...)
	if err != nil {
		return model.Gist{}, explain(err)
	}
	return g.model(), nil
}

// errInvalidUTF8 says a file cannot be sent through the API.
var errInvalidUTF8 = errors.New("the content is not valid UTF-8, which the GitHub API cannot carry; use gist clone")

// checkUTF8 fails on a string encoding/json would quietly change, replacing
// each invalid byte with U+FFFD.
func checkUTF8(ss ...*string) error {
	for _, s := range ss {
		if s != nil && !utf8.ValidString(*s) {
			return errInvalidUTF8
		}
	}
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
