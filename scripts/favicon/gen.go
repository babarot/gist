//go:build ignore

// Draws the favicons of the page from one 16x16 grid: the g of Redaction 70
// at its own pixels. Run it from the root of the repository:
//
//	go run scripts/favicon/gen.go
//
// docs/favicon/VARIANT holds the SVG, the ICO, a PNG of each size and the
// icon for a phone's home screen.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var g = []string{
	"................",
	"................",
	"..........##....",
	"......####.#....",
	".....##..##.....",
	".....##..##.....",
	".....##..##.....",
	".....######.....",
	"......####......",
	".....#..........",
	".....######.....",
	"......######....",
	"....##....##....",
	"....##....#.....",
	"................",
	"................",
}

var variants = []struct{ name, bg, fg string }{
	{"white", "#1e1e2e", "#e0e4f0"},
	{"green", "#1e1e2e", "#b5f0b0"},
	{"black", "#ffffff", "#1e1e2e"},
}

// sizes are multiples of 16, so that every pixel of the grid stays whole.
var sizes = []int{16, 32, 48, 64, 96, 192, 512}

const (
	root      = "docs/favicon"
	touchSize = 180
	touchCell = 10
)

// cell tells the color of a cell of the rounded tile: empty on its four
// corners, fg on the g, and bg elsewhere.
func cell(x, y int, bg, fg string) string {
	last := len(g) - 1
	if (x == 0 || x == last) && (y == 0 || y == last) {
		return ""
	}
	if g[y][x] == '#' {
		return fg
	}
	return bg
}

func rgba(hex string) color.NRGBA {
	if hex == "" {
		return color.NRGBA{}
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		log.Fatal(err)
	}
	return color.NRGBA{r, g, b, 255}
}

// tile draws the grid with each cell a square of the given side.
func tile(side int, bg, fg string) *image.NRGBA {
	n := len(g)
	img := image.NewNRGBA(image.Rect(0, 0, n*side, n*side))
	for y := range n * side {
		for x := range n * side {
			img.SetNRGBA(x, y, rgba(cell(x/side, y/side, bg, fg)))
		}
	}
	return img
}

// touch draws the icon for a home screen: square, since the phone rounds
// it, with a cell of margin.
func touch(bg, fg string) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, touchSize, touchSize))
	margin := (touchSize - len(g)*touchCell) / 2
	for y := range touchSize {
		for x := range touchSize {
			c := bg
			cx, cy := (x-margin)/touchCell, (y-margin)/touchCell
			if x >= margin && y >= margin && cx < len(g) && cy < len(g) && g[cy][cx] == '#' {
				c = fg
			}
			img.SetNRGBA(x, y, rgba(c))
		}
	}
	return img
}

func encode(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

// ico wraps PNGs into an ICO, which is a directory of images.
func ico(pngs map[int][]byte, order []int) []byte {
	var b bytes.Buffer
	put := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			log.Fatal(err)
		}
	}
	put([]uint16{0, 1, uint16(len(order))})
	offset := 6 + 16*len(order)
	for _, size := range order {
		data := pngs[size]
		b.Write([]byte{byte(size), byte(size), 0, 0})
		put([]uint16{1, 32})
		put([]uint32{uint32(len(data)), uint32(offset)})
		offset += len(data)
	}
	for _, size := range order {
		b.Write(pngs[size])
	}
	return b.Bytes()
}

// svg writes the tile as runs of cells, one path for each color.
func svg(bg, fg string) []byte {
	paths := map[string]*strings.Builder{bg: {}, fg: {}}
	for y := range g {
		for x := 0; x < len(g[y]); {
			c := cell(x, y, bg, fg)
			end := x + 1
			for end < len(g[y]) && cell(end, y, bg, fg) == c {
				end++
			}
			if c != "" {
				fmt.Fprintf(paths[c], "M%d %dh%dv1h-%dz", x, y, end-x, end-x)
			}
			x = end
		}
	}
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" shape-rendering="crispEdges"><path fill="%s" d="%s"/><path fill="%s" d="%s"/></svg>`+"\n",
		bg, paths[bg], fg, paths[fg])
}

func write(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	for _, v := range variants {
		dir := filepath.Join(root, v.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
		pngs := map[int][]byte{}
		for _, size := range sizes {
			pngs[size] = encode(tile(size/len(g), v.bg, v.fg))
			write(filepath.Join(dir, fmt.Sprintf("favicon-%d.png", size)), pngs[size])
		}
		write(filepath.Join(dir, "favicon.svg"), svg(v.bg, v.fg))
		write(filepath.Join(dir, "favicon.ico"), ico(pngs, []int{16, 32, 48}))
		write(filepath.Join(dir, "apple-touch-icon.png"), encode(touch(v.bg, v.fg)))
	}
}
