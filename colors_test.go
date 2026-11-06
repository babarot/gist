package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// colorsAllowed are where a color may be written: the themes, each a
// palette of colors, and mixing two of a palette's colors into a third.
// Everything else takes its colors from the palette of the theme shown, so
// that a color written anywhere else does not stay the same on every theme.
var colorsAllowed = map[string]string{
	"internal/tui/themes.go": "",    // the whole file
	"internal/tui/styles.go": "mix", // the function
}

var (
	// hexColor is a color written as a string, such as "#1e1e2e".
	hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	// sgrColor is an escape sequence that sets a color itself: a
	// foreground or background, of the 16 colors, of 256, or of RGB.
	sgrColor = regexp.MustCompile(`\x1b\[(?:[0-9]*;)*(?:3[0-8]|4[0-8]|9[0-7]|10[0-7])(?:;[0-9]*)*m`)
	// colorFuncs are the calls and types that make a color, by package
	// and name.
	colorFuncs = map[string]bool{
		"lipgloss.Color": true, "lipgloss.ANSIColor": true, "lipgloss.RGBColor": true,
		"color.RGBA": true, "color.NRGBA": true, "color.RGBA64": true, "color.NRGBA64": true, "color.Gray": true, "color.Gray16": true,
		"ansi.BasicColor": true, "ansi.IndexedColor": true, "ansi.TrueColor": true, "ansi.RGBColor": true,
	}
)

// TestColorsOnlyInThemes fails on a color written outside colorsAllowed,
// in the code under internal/ that is not a test.
func TestColorsOnlyInThemes(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir("internal", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		fn, allowed := colorsAllowed[filepath.ToSlash(path)]
		if allowed && fn == "" {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && allowed && fd.Recv == nil && fd.Name.Name == fn {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if what := colorIn(n); what != "" {
					t.Errorf("%s: %s; a color belongs in a theme in internal/tui/themes.go", fset.Position(n.Pos()), what)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// colorIn says what color n writes, or "".
func colorIn(n ast.Node) string {
	switch n := n.(type) {
	case *ast.BasicLit:
		if n.Kind != token.STRING {
			return ""
		}
		s, err := strconv.Unquote(n.Value)
		if err != nil {
			return ""
		}
		if hexColor.MatchString(s) {
			return "the color " + n.Value
		}
		if sgrColor.MatchString(s) {
			return "an escape sequence that sets a color, " + n.Value
		}
	case *ast.CallExpr:
		if name := selectorName(n.Fun); colorFuncs[name] {
			return name + "(...)"
		}
	case *ast.CompositeLit:
		if name := selectorName(n.Type); colorFuncs[name] {
			return name + "{...}"
		}
	}
	return ""
}

// selectorName is pkg.Name for an expression of that form, or "".
func selectorName(e ast.Expr) string {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name + "." + sel.Sel.Name
}
