package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/babarot/gist/v2/internal/model"
)

// ListOptions are the flags of gist list.
type ListOptions struct {
	Public, Secret, Starred, Refresh, JSON bool
	Limit                                  int
}

// List prints the list of gists.
func (c *CLI) List(ctx context.Context, o ListOptions) error {
	var gists []model.Gist
	var err error
	if o.Starred {
		gists, err = c.starred(ctx, o.Refresh)
	} else {
		gists, err = c.mine(ctx, o.Refresh)
	}
	if err != nil {
		return err
	}
	shown := make([]model.Gist, 0, len(gists))
	for _, g := range gists {
		if (o.Public && !g.Public) || (o.Secret && g.Public) {
			continue
		}
		if o.Limit > 0 && len(shown) == o.Limit {
			break
		}
		shown = append(shown, g)
	}
	if o.JSON {
		return c.printJSON(shown)
	}
	if c.StdoutTTY {
		c.printTable(shown)
		return nil
	}
	for _, g := range shown {
		fmt.Fprintf(c.Stdout, "%s\t%s\t%d\t%d\t%s\t%s\n", g.ID, oneLine(g.Description), len(g.Files), g.Comments,
			visibility(g), g.Updated.Format(time.RFC3339))
	}
	return nil
}

func visibility(g model.Gist) string {
	if g.Public {
		return "public"
	}
	return "secret"
}

const shortID = 7

// printTable lines up the list for a person, with an ID as short as is shown
// elsewhere and how long ago each gist changed.
func (c *CLI) printTable(gists []model.Gist) {
	width := len("DESCRIPTION")
	lead := make([]string, len(gists))
	for i, g := range gists {
		lead[i] = oneLine(g.Description)
		if lead[i] == "" && len(g.Files) > 0 {
			lead[i] = oneLine(g.Files[0].Name)
		}
		width = max(width, min(len([]rune(lead[i])), 60))
	}
	fmt.Fprintf(c.Stdout, "%-7s  %-*s  %5s  %8s  %-6s  %s\n", "ID", width, "DESCRIPTION", "FILES", "COMMENTS", "VIS", "UPDATED")
	for i, g := range gists {
		comments := ""
		if g.Comments > 0 {
			comments = fmt.Sprint(g.Comments)
		}
		fmt.Fprintf(c.Stdout, "%-7s  %-*s  %5d  %8s  %-6s  %s\n", g.ID[:min(shortID, len(g.ID))], width, cut(lead[i], 60),
			len(g.Files), comments, visibility(g), ago(c.Now().Sub(g.Updated)))
	}
}

// cut shortens s to n characters.
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ago says how long d is, shortly.
func ago(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < day:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 30*day:
		return fmt.Sprintf("%dd", int(d/day))
	case d < 365*day:
		return fmt.Sprintf("%dmo", int(d/(30*day)))
	}
	return fmt.Sprintf("%dy", int(d/(365*day)))
}

func (c *CLI) printJSON(v any) error {
	e := json.NewEncoder(c.Stdout)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}
