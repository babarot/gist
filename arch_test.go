package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// The packages and what each may import, from the bottom layer up. A
// package imports only from lower layers and only the outside modules
// listed for it; the parts of the standard library that reach outside the
// process belong to a few packages each. docs/architecture.md shows this
// table, and TestArchitectureDoc keeps the two the same. Test files are
// not checked: a test may import what helps it.
//
// A package not in the table fails the test, so a new one is placed here
// on purpose.
var layers = []layer{
	{"internal/model", 0, nil, nil, "Shared types: gists, their files and comments, and changes to them"},
	{"internal/version", 0, nil, nil, "The release version, which tagpr updates"},
	{"internal/xdg", 0, nil, nil, "Where gist keeps its config, state and cache"},
	{"internal/state", 0, nil, nil, "What would be missed if lost: drafts, the locks that guard them, and the layout"},
	{"internal/near", 0, nil, nil, "The closest of a few names to one mistyped, for a did-you-mean"},
	{"internal/text", 0, nil, []string{"github.com/charmbracelet/x/ansi", "github.com/rivo/uniseg"}, "Making text from GitHub fit to draw, and its width"},
	{"internal/diff", 0, nil, []string{"github.com/pmezard/go-difflib"}, "What changed between two versions of a file"},
	{"internal/filter", 0, nil, nil, "Fuzzy matching and the key:value terms of the filter"},
	{"internal/release", 0, nil, nil, "Whether a newer release is out, how to update, and what its notes say"},
	{"internal/fakegist", 0, nil, nil, "A made-up GitHub Gist API for tests; never built into gist"},
	{"internal/tui/statusline", 0, nil, nil, "What the status line shows: events for a moment, then work in progress"},
	{"internal/highlight", 0, nil, []string{"github.com/alecthomas/chroma/v2"}, "A file as lines of tokens, each saying what it is"},
	{"internal/logging", 1, []string{"internal/xdg"}, []string{"github.com/nxadm/tail"}, "The slog logger and the log file"},
	{"internal/cache", 1, []string{"internal/model"}, nil, "What was read from GitHub, kept for the next launch"},
	{"internal/resolve", 1, []string{"internal/model"}, nil, "What the user calls a gist, turned into its ID"},
	{"internal/edit", 1, []string{"internal/model", "internal/state"}, nil, "Changing a gist: the lock, the base, drafts, and checking it did not move"},
	{"internal/config", 1, []string{"internal/near"}, []string{"github.com/pelletier/go-toml/v2"}, "Reading the config file"},
	{"internal/tui/keymap", 1, []string{"internal/near"}, nil, "Keys bound to actions by name"},
	{"internal/markdown", 1, []string{"internal/text"}, []string{"github.com/kyokomi/emoji/v2", "golang.org/x/net/html"}, "Markdown and HTML drawn as lines, each piece saying what it is"},
	{"internal/github", 2, []string{"internal/model", "internal/logging"}, []string{"github.com/cli/go-gh/v2"}, "GitHub's REST API"},
	{"internal/cli", 3, []string{"internal/cache", "internal/diff", "internal/edit", "internal/github", "internal/model", "internal/resolve"}, nil, "What the subcommands do"},
	{"internal/tui", 3, []string{"internal/cache", "internal/config", "internal/diff", "internal/edit", "internal/filter", "internal/github", "internal/highlight", "internal/markdown", "internal/model", "internal/release", "internal/state", "internal/text", "internal/tui/keymap", "internal/tui/statusline"},
		[]string{"charm.land/bubbles/v2", "charm.land/bubbletea/v2", "charm.land/lipgloss/v2", "github.com/rivo/uniseg"}, "The terminal UI"},
	{"internal/app", 4, []string{"internal/cli", "internal/config", "internal/edit", "internal/github", "internal/logging", "internal/model", "internal/release", "internal/state", "internal/tui", "internal/tui/keymap", "internal/version", "internal/xdg"}, []string{"github.com/cli/go-gh/v2", "github.com/spf13/cobra"}, "Putting it together: commands and flags, the log, and what each command runs with"},
	{"", 5, []string{"internal/app"}, nil, "Calls app.Run"},
	{"demo/internal/recording", 4, []string{"internal/config", "internal/edit", "internal/fakegist", "internal/github", "internal/logging", "internal/model", "internal/state", "internal/tui", "internal/tui/keymap"}, nil, "The recording in the README: the TUI against a made-up GitHub"},
	{"demo/recording", 5, []string{"demo/internal/recording"}, nil, "Runs the recording"},
}

// layer is a package's place: its layer, the packages of this module
// (named by their path in it) and outside modules it may import, and what
// it does.
type layer struct {
	pkg      string
	layer    int
	local    []string
	external []string
	does     string
}

// reach are the packages that only some may use: the parts of the standard
// library that reach outside the process, and internal/text, whose cleaning
// is for what is drawn. Anywhere else it would change a gist's content without
// showing in a diff. An entry may name a package not written yet.
var reach = map[string][]string{
	"internal/text": {"internal/markdown", "internal/tui"},
	"net":           {"internal/fakegist", "internal/github"},
	"net/http":      {"internal/fakegist", "internal/github"},
	"os/exec":       {"internal/app", "internal/cli", "internal/tui"},
	"syscall":       {"internal/state"},
}

const module = "github.com/babarot/gist/v2"

type goPackage struct {
	ImportPath string
	Imports    []string
}

func listPackages(t *testing.T) []goPackage {
	t.Helper()
	out, err := exec.Command("go", "list", "-json", "./...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []goPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func TestArchitecture(t *testing.T) {
	for _, p := range listPackages(t) {
		rel := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, module), "/")
		i := slices.IndexFunc(layers, func(l layer) bool { return l.pkg == rel })
		if i < 0 {
			t.Errorf("%s is not in the layer table in arch_test.go and docs/architecture.md", p.ImportPath)
			continue
		}
		l := layers[i]
		for _, imp := range p.Imports {
			switch {
			case strings.HasPrefix(imp, module+"/"):
				dep := strings.TrimPrefix(imp, module+"/")
				if !slices.Contains(l.local, dep) {
					t.Errorf("%s imports %s, which is not in its allowed imports", p.ImportPath, imp)
				}
				if who, ok := reach[dep]; ok && !slices.Contains(who, rel) {
					t.Errorf("%s imports %s, which only %s may use", p.ImportPath, imp, strings.Join(who, ", "))
				}
			case strings.Contains(strings.SplitN(imp, "/", 2)[0], "."):
				if !slices.ContainsFunc(l.external, func(m string) bool { return imp == m || strings.HasPrefix(imp, m+"/") }) {
					t.Errorf("%s imports %s, an outside module not allowed for it", p.ImportPath, imp)
				}
			default:
				if who, ok := reach[imp]; ok && !slices.Contains(who, rel) {
					t.Errorf("%s imports %s, which only %s may use", p.ImportPath, imp, strings.Join(who, ", "))
				}
			}
		}
	}
}

// TestLayersGoDownward checks the table itself: every allowed import is a
// package of a lower layer.
func TestLayersGoDownward(t *testing.T) {
	layerOf := map[string]int{}
	for _, l := range layers {
		layerOf[l.pkg] = l.layer
	}
	for _, l := range layers {
		for _, dep := range l.local {
			dl, ok := layerOf[dep]
			if !ok || dl >= l.layer {
				t.Errorf("%s (layer %d) may import %s (layer %d, known %v)", l.pkg, l.layer, dep, dl, ok)
			}
		}
	}
}

// TestNoUnusedImportAllowed checks that every package of the table exists
// and every import it allows is made, so the table tells what each
// package depends on. An outside
// module is used when the module or a package in it is imported. reach is
// not checked: it limits who may use a package, and a package no one uses
// is still limited.
func TestNoUnusedImportAllowed(t *testing.T) {
	imports := map[string][]string{}
	for _, p := range listPackages(t) {
		rel := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, module), "/")
		imports[rel] = p.Imports
	}
	for _, l := range layers {
		imps, ok := imports[l.pkg]
		if !ok {
			t.Errorf("%s is in the table but is not a package", pkgName(l.pkg))
			continue
		}
		for _, dep := range l.local {
			if !slices.Contains(imps, module+"/"+dep) {
				t.Errorf("%s may import %s but does not; remove it from the table", pkgName(l.pkg), dep)
			}
		}
		for _, m := range l.external {
			if !slices.ContainsFunc(imps, func(imp string) bool { return imp == m || strings.HasPrefix(imp, m+"/") }) {
				t.Errorf("%s may import %s but does not; remove it from the table", pkgName(l.pkg), m)
			}
		}
	}
}

// pkgName is a package of the table as people call it.
func pkgName(pkg string) string {
	if pkg == "" {
		return "main"
	}
	return pkg
}

var update = flag.Bool("update", false, "rewrite the layer table in docs/architecture.md")

const (
	docPath  = "docs/architecture.md"
	docStart = "<!-- layers:start -->"
	docEnd   = "<!-- layers:end -->"
)

// docTable is the layer table as docs/architecture.md shows it.
func docTable() string {
	var b strings.Builder
	b.WriteString("| Layer | Package | Does | May import | Outside modules |\n|---|---|---|---|---|\n")
	for _, l := range layers {
		name := "`" + l.pkg + "`"
		if l.pkg == "" {
			name = "`main`"
		}
		join := func(ss []string) string {
			if len(ss) == 0 {
				return "-"
			}
			q := make([]string, len(ss))
			for i, s := range ss {
				q[i] = "`" + s + "`"
			}
			return strings.Join(q, ", ")
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n", l.layer, name, l.does, join(l.local), join(l.external))
	}
	b.WriteString("\nPackages only some may use:\n\n| Package | Only in |\n|---|---|\n")
	names := make([]string, 0, len(reach))
	for n := range reach {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		who := make([]string, len(reach[n]))
		for i, w := range reach[n] {
			who[i] = "`" + w + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s |\n", n, strings.Join(who, ", "))
	}
	return b.String()
}

func TestArchitectureDoc(t *testing.T) {
	src, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(src)
	start, end := strings.Index(doc, docStart), strings.Index(doc, docEnd)
	if start < 0 || end < start {
		t.Fatalf("%s has no %s ... %s block", docPath, docStart, docEnd)
	}
	want := docStart + "\n" + docTable() + docEnd
	got := doc[start : end+len(docEnd)]
	if got == want {
		return
	}
	if *update {
		if err := os.WriteFile(docPath, []byte(doc[:start]+want+doc[end+len(docEnd):]), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Errorf("the layer table in %s differs from arch_test.go; run go test -run TestArchitectureDoc -update", docPath)
}
