# Architecture

The packages are layered: a package imports only from lower layers, and only the outside modules listed for it. `arch_test.go` checks every import against this table. To change it, edit the table there and run `go test -run TestArchitectureDoc -update`.

<!-- layers:start -->
| Layer | Package | Does | May import | Outside modules |
|---|---|---|---|---|
| 0 | `internal/model` | Shared types: gists, their files and comments, and changes to them | - | - |
| 0 | `internal/version` | The release version, which tagpr updates | - | - |
| 0 | `internal/xdg` | Where gist keeps its config, state and cache | - | - |
| 0 | `internal/state` | What would be missed if lost: drafts, and the locks that guard them | - | - |
| 0 | `internal/fakegist` | A made-up GitHub Gist API for tests; never built into gist | - | - |
| 1 | `internal/logging` | The slog logger and the log file | `internal/xdg` | `github.com/nxadm/tail` |
| 1 | `internal/cache` | What was read from GitHub, kept for the next launch | `internal/model` | - |
| 1 | `internal/resolve` | What the user calls a gist, turned into its ID | `internal/model` | - |
| 1 | `internal/edit` | Changing a gist: the lock, the base, drafts, and checking it did not move | `internal/model`, `internal/state` | - |
| 2 | `internal/github` | GitHub's REST API | `internal/model`, `internal/logging` | `github.com/cli/go-gh/v2` |
| 3 | `internal/cli` | What the subcommands do | `internal/cache`, `internal/edit`, `internal/github`, `internal/model`, `internal/resolve` | - |
| 4 | `internal/app` | Putting it together: commands and flags, the log, and what each command runs with | `internal/cli`, `internal/edit`, `internal/github`, `internal/logging`, `internal/state`, `internal/version`, `internal/xdg` | `github.com/cli/go-gh/v2`, `github.com/spf13/cobra` |
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
