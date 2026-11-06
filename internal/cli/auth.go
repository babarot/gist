// Package cli holds what the subcommands do, once internal/app has parsed
// their arguments.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/babarot/gist/v2/internal/github"
)

// AccessChecker says what the token can do.
type AccessChecker interface {
	CheckAccess(ctx context.Context) (github.Access, error)
}

// Auth prints where the token came from and what it can do, and returns the
// exit code: 1 when it cannot be used for gists.
func Auth(ctx context.Context, stdout, stderr io.Writer, source string, c AccessChecker) int {
	a, err := c.CheckAccess(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "gist: %v\n", err)
		return 1
	}
	access, code := "read (a fine-grained token needs the Gists read and write permission to write; not checked)", 0
	if a.Scoped {
		access = "read and write"
		if !a.GistScope {
			access, code = "none: the token lacks the gist scope", 1
		}
	}
	fmt.Fprintf(stdout, "host   %s\nuser   %s\ntoken  %s\ngists  %s\n", github.Host, a.Login, github.Source(source), access)
	return code
}
