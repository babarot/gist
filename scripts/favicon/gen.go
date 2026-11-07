//go:build ignore

// Draws the favicons of the page: the g of Redaction 70 at its own pixels, on
// a moon dithered to ink and paper. Run it from the root of the repository:
//
//	go run scripts/favicon/gen.go
//
// docs/favicon/VARIANT holds the SVG, the ICO, a PNG of each size and the
// icon for a phone's home screen. Each is drawn on a grid of 16 or 32, whichever
// the size is a multiple of, so that every pixel stays whole.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	ink   = "#05060c"
	paper = "#e6e8f2"

	root      = "docs/favicon"
	touchSize = 180
)

var glyph = []string{
	"......##",
	"..####.#",
	".##..##.",
	".##..##.",
	".##..##.",
	".######.",
	"..####..",
	".#......",
	".######.",
	"..######",
	"##....##",
	"##....#.",
}

var bayer = [16]float64{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

type grid [][]string

func newGrid(n int, bg string) grid {
	g := make(grid, n)
	for y := range g {
		g[y] = make([]string, n)
		for x := range g[y] {
			g[y][x] = bg
		}
	}
	corner := max(1, n/16)
	for i := range corner {
		for j := range corner - i {
			g[j][i], g[j][n-1-i], g[n-1-j][i], g[n-1-j][n-1-i] = "", "", "", ""
		}
	}
	return g
}

type crater struct{ x, y, r float64 }

type moon struct {
	x, y, r    float64 // as parts of the grid
	light      [3]float64
	base, gain float64
	color      string
	craters    []crater
}

// lit draws what of the sphere the light reaches, ordered dithered.
func (m moon) lit(g grid) {
	n := float64(len(g))
	for y := range g {
		for x := range g[y] {
			if g[y][x] == "" {
				continue
			}
			px, py := float64(x)+0.5, float64(y)+0.5
			nx, ny := (px-m.x*n)/(m.r*n), (py-m.y*n)/(m.r*n)
			d := nx*nx + ny*ny
			if d >= 1 {
				continue
			}
			nz := math.Sqrt(1 - d)
			v := m.base + m.gain*max(0, nx*m.light[0]+ny*m.light[1]+nz*m.light[2])
			for _, c := range m.craters {
				q := math.Hypot(px-c.x*n, py-c.y*n) / (c.r * n)
				if q >= 1 {
					continue
				}
				if q < 0.75 {
					v *= 0.15
				} else {
					v *= 1.5
				}
			}
			if v > (bayer[x&3+(y&3)*4]+0.5)/16 {
				g[y][x] = m.color
			}
		}
	}
}

// letter draws the g in ink, with a line of paper around it.
func letter(g grid) {
	n := len(g)
	k := max(1, n/16)
	x0, y0 := 4, n/8
	if k > 1 {
		x0 = n / 4
	}
	type point struct{ x, y int }
	var points []point
	for y, row := range glyph {
		for x, c := range row {
			if c != '#' {
				continue
			}
			for a := range k {
				for b := range k {
					points = append(points, point{x0 + x*k + a, y0 + y*k + b})
				}
			}
		}
	}
	for _, p := range points {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				x, y := p.x+dx, p.y+dy
				if y >= 0 && y < n && x >= 0 && x < n && g[y][x] != "" {
					g[y][x] = paper
				}
			}
		}
	}
	for _, p := range points {
		g[p.y][p.x] = ink
	}
}

var variants = []struct {
	name, bg string
	moon     moon
}{
	{"moon", ink, moon{
		x: 0.5, y: 0.5, r: 0.49, light: [3]float64{-0.45, -0.65, 0.6},
		base: 0.45, gain: 0.9, color: paper,
	}},
	{"surface", paper, moon{
		x: 0.5, y: 0.5, r: 0.95, light: [3]float64{0.5, 0.6, -0.5},
		base: 0.05, gain: 0.75, color: ink,
		craters: []crater{{0.2, 0.75, 0.12}, {0.83, 0.2, 0.1}},
	}},
}

func draw(n int, bg string, m moon) grid {
	g := newGrid(n, bg)
	m.lit(g)
	letter(g)
	return g
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

func (g grid) png(side int) []byte {
	n := len(g)
	img := image.NewNRGBA(image.Rect(0, 0, n*side, n*side))
	for y := range n * side {
		for x := range n * side {
			img.SetNRGBA(x, y, rgba(g[y/side][x/side]))
		}
	}
	return encode(img)
}

// touch is the icon for a home screen: square, since the phone rounds it.
func (g grid) touch(bg string) []byte {
	n := len(g)
	side := touchSize / n
	margin := (touchSize - n*side) / 2
	img := image.NewNRGBA(image.Rect(0, 0, touchSize, touchSize))
	for y := range touchSize {
		for x := range touchSize {
			c := bg
			cx, cy := x-margin, y-margin
			if cx >= 0 && cy >= 0 && cx/side < n && cy/side < n && g[cy/side][cx/side] != "" {
				c = g[cy/side][cx/side]
			}
			img.SetNRGBA(x, y, rgba(c))
		}
	}
	return encode(img)
}

func encode(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

// ico wraps PNGs into an ICO, which is a directory of images.
func ico(sizes []int, pngs map[int][]byte) []byte {
	var b bytes.Buffer
	put := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			log.Fatal(err)
		}
	}
	put([]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for _, size := range sizes {
		b.Write([]byte{byte(size), byte(size), 0, 0})
		put([]uint16{1, 32})
		put([]uint32{uint32(len(pngs[size])), uint32(offset)})
		offset += len(pngs[size])
	}
	for _, size := range sizes {
		b.Write(pngs[size])
	}
	return b.Bytes()
}

// svg writes the grid as runs of cells, one path for each color.
func (g grid) svg() []byte {
	paths := map[string]*strings.Builder{ink: {}, paper: {}}
	for y, row := range g {
		for x := 0; x < len(row); {
			end := x + 1
			for end < len(row) && row[end] == row[x] {
				end++
			}
			if row[x] != "" {
				fmt.Fprintf(paths[row[x]], "M%d %dh%dv1h-%dz", x, y, end-x, end-x)
			}
			x = end
		}
	}
	n := len(g)
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><path fill="%s" d="%s"/><path fill="%s" d="%s"/></svg>`+"\n",
		n, n, ink, paths[ink], paper, paths[paper])
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
		small := draw(16, v.bg, v.moon)
		large := draw(32, v.bg, v.moon)
		pngs := map[int][]byte{16: small.png(1), 48: small.png(3)}
		for _, size := range []int{32, 64, 96, 192, 512} {
			pngs[size] = large.png(size / 32)
		}
		for size, data := range pngs {
			write(filepath.Join(dir, fmt.Sprintf("favicon-%d.png", size)), data)
		}
		write(filepath.Join(dir, "favicon.svg"), large.svg())
		write(filepath.Join(dir, "favicon.ico"), ico([]int{16, 32, 48}, pngs))
		write(filepath.Join(dir, "apple-touch-icon.png"), large.touch(v.bg))
	}
}
