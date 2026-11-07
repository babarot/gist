package recording

import (
	"time"

	"github.com/babarot/gist/v2/internal/fakegist"
)

// Login is who the recording is logged in as.
const Login = "alice"

type gist struct {
	age         time.Duration
	description string
	public      bool
	files       map[string]string
	comments    []string
}

// Fill gives srv the recording's gists, aged from now, so that it reads the same
// whenever it is made. Oldest first, as a gist added last is the newest.
func Fill(srv *fakegist.Server, now time.Time) {
	srv.Login = Login
	srv.Releases = []fakegist.Release{
		{TagName: "v2.0.0", PublishedAt: now.Add(-3 * time.Hour), Body: notes},
		{TagName: "v1.2.8", PublishedAt: now.Add(-30 * 24 * time.Hour), Body: "## What's Changed\n* Show the version the release was built with by @alice in https://github.com/babarot/gist/pull/19\n"},
	}
	for _, g := range mine {
		srv.SetClock(now.Add(-g.age))
		made := srv.Add(g.description, g.public, g.files)
		for _, c := range g.comments {
			srv.Comment(made.ID, "mika-dev", c)
		}
	}
	srv.SetClock(now.Add(-9 * 24 * time.Hour))
	srv.Star("mika-dev", "Vim keys for tmux panes", map[string]string{"tmux-nav.conf": tmuxNav})
	srv.Star("rustacean42", "Lifetimes, in one page", map[string]string{"lifetimes.md": lifetimes})
	srv.Star("ops-notes", "kubectl one-liners", map[string]string{"kubectl.md": kubectl})
	srv.SetClock(now)
}

const notes = `## What's Changed
### New Features
* A TUI to read and edit your gists, with a scriptable CLI by @alice in https://github.com/babarot/gist/pull/25
* Filter by file, language and visibility, with tab completion by @alice in https://github.com/babarot/gist/pull/36
* Check a diff before it is pushed by @alice in https://github.com/babarot/gist/pull/30
`

var mine = []gist{
	{400 * 24 * time.Hour, "Dockerfile for a small Go service", true, map[string]string{"Dockerfile": dockerfile}, nil},
	{120 * 24 * time.Hour, "A tiny static file server", true, map[string]string{"server.py": serverPy}, nil},
	{60 * 24 * time.Hour, "Debug pod for a cluster", false, map[string]string{"debug.yaml": debugYAML}, nil},
	{45 * 24 * time.Hour, "Slowest queries of the last hour", false, map[string]string{"slow.sql": slowSQL}, nil},
	{30 * 24 * time.Hour, "Notes on Go generics", true, map[string]string{"generics.md": generics}, nil},
	{14 * 24 * time.Hour, "Shell tricks I keep forgetting", false, map[string]string{"tips.sh": tipsSh}, nil},
	{7 * 24 * time.Hour, "nginx in front of a dev server", false, map[string]string{"nginx.conf": nginx}, nil},
	{3 * 24 * time.Hour, "zsh helpers for git", true, map[string]string{"README.md": zshReadme, "fzf-branch.zsh": zshBranch, "fzf-log.zsh": zshLog}, nil},
	{20 * time.Hour, "Retry an HTTP request with backoff", true, map[string]string{"retry.go": retryGo}, []string{
		"Should a POST be retried here too? It may not be safe to send twice.",
		"`rand.N` is a nice touch: the jitter keeps clients from retrying together.",
	}},
	{2 * time.Hour, "Notes from the weekly meeting", false, map[string]string{"minutes.md": minutes}, nil},
}

const dockerfile = `FROM golang:1.26 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /app .

FROM gcr.io/distroless/static
COPY --from=build /app /app
ENTRYPOINT ["/app"]
`

const serverPy = `import http.server
import socketserver

PORT = 8000

with socketserver.TCPServer(("", PORT), http.server.SimpleHTTPRequestHandler) as httpd:
    print(f"serving on {PORT}")
    httpd.serve_forever()
`

const debugYAML = `apiVersion: v1
kind: Pod
metadata:
  name: debug
spec:
  containers:
    - name: shell
      image: nicolaka/netshoot
      command: ["sleep", "infinity"]
`

const slowSQL = `SELECT query, calls, mean_exec_time
FROM pg_stat_statements
WHERE last_exec > now() - interval '1 hour'
ORDER BY mean_exec_time DESC
LIMIT 20;
`

const generics = "# Notes on Go generics\n\nA type parameter is worth it when the code would be the same for each type.\n\n" +
	"## Constraints\n\n```go\ntype Number interface {\n\t~int | ~int64 | ~float64\n}\n\nfunc Sum[T Number](xs []T) T {\n\tvar total T\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn total\n}\n```\n\n" +
	"## When not to\n\n- One type uses it: write that type.\n- The methods differ: use an interface.\n"

const tipsSh = `#!/bin/sh
# Reuse the last argument of the previous command
mkdir -p ~/work/demo && cd "$_"

# Check only the files that changed
git diff --name-only | xargs shellcheck

# The ten largest files
du -ah . | sort -rh | head -n 10
`

const nginx = `server {
    listen 80;
    server_name dev.local;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
`

const zshReadme = "# zsh helpers for git\n\nTwo widgets, on top of [fzf](https://github.com/junegunn/fzf):\n\n" +
	"| Key | |\n|---|---|\n| `ctrl-g b` | switch to a branch |\n| `ctrl-g l` | look through the log |\n\n" +
	"Source the files from your `.zshrc`.\n"

const zshBranch = `fzf-branch() {
  local branch
  branch=$(git branch --format='%(refname:short)' | fzf --height 40%) || return
  git switch "$branch"
  zle reset-prompt
}
zle -N fzf-branch
bindkey '^gb' fzf-branch
`

const zshLog = `fzf-log() {
  git log --oneline --color=always |
    fzf --ansi --preview 'git show --color=always {1}'
  zle reset-prompt
}
zle -N fzf-log
bindkey '^gl' fzf-log
`

const retryGo = `package retry

import (
	"context"
	"math/rand/v2"
	"net/http"
	"time"
)

// Do sends req until it succeeds, waiting longer after each failure.
func Do(ctx context.Context, c *http.Client, req *http.Request, tries int) (*http.Response, error) {
	var last error
	for i := range tries {
		res, err := c.Do(req.Clone(ctx))
		if err == nil && res.StatusCode < 500 {
			return res, nil
		}
		last = err
		wait := time.Duration(1<<i)*100*time.Millisecond + rand.N(50*time.Millisecond)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, last
}
`

// Line holds the long line the recording's edit changes at its end, past what a
// row shows.
const minutes = `# Notes from the weekly meeting

## Decided

- Search is too slow, so we will review the indexes first, bring the numbers from production to next week's meeting, and decide there whether a cache is worth adding.

## Before the next one

- [ ] Write down how to measure it
- [x] Collect the topics
`

const tmuxNav = `bind h select-pane -L
bind j select-pane -D
bind k select-pane -U
bind l select-pane -R
`

const lifetimes = "# Lifetimes, in one page\n\nA lifetime names how long a reference stays valid.\n\n" +
	"```rust\nfn longest<'a>(x: &'a str, y: &'a str) -> &'a str {\n    if x.len() > y.len() { x } else { y }\n}\n```\n"

const kubectl = "# kubectl one-liners\n\nFind the pods that restart the most.\n\n" +
	"```sh\nkubectl get pods -A --sort-by=.status.containerStatuses[0].restartCount\n```\n"
