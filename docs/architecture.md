# Architecture

The packages are layered: a package imports only from lower layers, and only the outside modules listed for it. `arch_test.go` checks every import against this table. To change it, edit the table there and run `go test -run TestArchitectureDoc -update`.

<!-- layers:start -->
| Layer | Package | Does | May import | Outside modules |
|---|---|---|---|---|
| 0 | `internal/model` | Shared types: gists, their files and comments, and changes to them | - | - |
| 0 | `internal/version` | The release version, which tagpr updates | - | - |
| 0 | `internal/xdg` | Where gist keeps its config, state and cache | - | - |
| 0 | `internal/state` | What would be missed if lost: drafts, the locks that guard them, and the layout | - | - |
| 0 | `internal/near` | The closest of a few names to one mistyped, for a did-you-mean | - | - |
| 0 | `internal/text` | Making text from GitHub fit to draw, and its width | - | `github.com/charmbracelet/x/ansi` |
| 0 | `internal/diff` | What changed between two versions of a file | - | `github.com/pmezard/go-difflib` |
| 0 | `internal/filter` | Fuzzy matching and the key:value terms of the filter | - | - |
| 0 | `internal/release` | Whether a newer release is out, how to update, and what its notes say | - | - |
| 0 | `internal/fakegist` | A made-up GitHub Gist API for tests; never built into gist | - | - |
| 0 | `internal/tui/statusline` | What the status line shows: events for a moment, then work in progress | - | - |
| 0 | `internal/highlight` | A file as lines of tokens, each saying what it is | - | `github.com/alecthomas/chroma/v2` |
| 1 | `internal/logging` | The slog logger and the log file | `internal/xdg` | `github.com/nxadm/tail` |
| 1 | `internal/cache` | What was read from GitHub, kept for the next launch | `internal/model` | - |
| 1 | `internal/resolve` | What the user calls a gist, turned into its ID | `internal/model` | - |
| 1 | `internal/edit` | Changing a gist: the lock, the base, drafts, and checking it did not move | `internal/model`, `internal/state` | - |
| 1 | `internal/config` | Reading the config file | `internal/near` | `github.com/pelletier/go-toml/v2` |
| 1 | `internal/tui/keymap` | Keys bound to actions by name | `internal/near` | - |
| 1 | `internal/markdown` | Markdown and HTML drawn as lines, each piece saying what it is | `internal/text` | `github.com/kyokomi/emoji/v2`, `golang.org/x/net/html` |
| 2 | `internal/github` | GitHub's REST API | `internal/model`, `internal/logging` | `github.com/cli/go-gh/v2` |
| 3 | `internal/cli` | What the subcommands do | `internal/cache`, `internal/diff`, `internal/edit`, `internal/github`, `internal/model`, `internal/resolve` | - |
| 3 | `internal/tui` | The terminal UI | `internal/cache`, `internal/config`, `internal/diff`, `internal/edit`, `internal/filter`, `internal/github`, `internal/highlight`, `internal/markdown`, `internal/model`, `internal/release`, `internal/state`, `internal/text`, `internal/tui/keymap`, `internal/tui/statusline` | `charm.land/bubbles/v2`, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `github.com/rivo/uniseg` |
| 4 | `internal/app` | Putting it together: commands and flags, the log, and what each command runs with | `internal/cli`, `internal/config`, `internal/edit`, `internal/github`, `internal/logging`, `internal/model`, `internal/release`, `internal/state`, `internal/tui`, `internal/tui/keymap`, `internal/version`, `internal/xdg` | `github.com/cli/go-gh/v2`, `github.com/spf13/cobra` |
| 5 | `main` | Calls app.Run | `internal/app` | - |

Packages only some may use:

| Package | Only in |
|---|---|
| `internal/text` | `internal/markdown`, `internal/tui` |
| `net` | `internal/fakegist`, `internal/github`, `internal/update` |
| `net/http` | `internal/fakegist`, `internal/github`, `internal/update` |
| `os/exec` | `internal/app`, `internal/cli`, `internal/tui` |
| `syscall` | `internal/state` |
<!-- layers:end -->
