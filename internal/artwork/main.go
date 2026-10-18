// Command artwork generates the README images from the public rendering API.
// Run from the repository root: go run ./internal/artwork [-check].
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/rickthegreatest/boxmaid"
)

const (
	cellWidth  = 10
	lineHeight = 23
	background = "#0b1220"
	foreground = "#e6edf6"
)

type example struct {
	path    string
	picture *boxmaid.Picture
}

func load(path string) (example, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return example{}, err
	}
	pic, err := boxmaid.Render(string(src), boxmaid.Options{Width: 100})
	if err != nil {
		return example{}, fmt.Errorf("%s: %w", path, err)
	}
	return example{path, pic}, nil
}

func start(b *strings.Builder, height int, title, description string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="%d" viewBox="0 0 1200 %d" role="img" aria-labelledby="title description">
<title id="title">%s</title><desc id="description">%s</desc>
<rect width="1200" height="%d" rx="24" fill="%s"/>
<g font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" fill="%s">
`, height, height, html.EscapeString(title), html.EscapeString(description), height, background, foreground)
}

func text(b *strings.Builder, x, y, size int, color, value string) {
	fmt.Fprintf(b, `<text x="%d" y="%d" font-size="%d" fill="%s">%s</text>`+"\n", x, y, size, color, html.EscapeString(value))
}

func panelHeight(e example) int { return 115 + e.picture.H*lineHeight }

func panel(b *strings.Builder, e example, x, y, w int) {
	h := panelHeight(e)
	fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" rx="14" fill="#111d30" stroke="#28384e"/>`+"\n", x, y, w, h)
	for i, color := range []string{"#fb7185", "#fbbf24", "#34d399"} {
		fmt.Fprintf(b, `<circle cx="%d" cy="%d" r="5" fill="%s"/>`+"\n", x+24+i*18, y+25, color)
	}
	text(b, x+92, y+31, 14, "#9aadc5", "boxmaid -theme nord "+filepath.Base(e.path))
	theme := boxmaid.NordTheme
	// Increase contrast against the artwork's background while retaining role colours.
	theme.Styles = maps.Clone(theme.Styles)
	theme.Styles[boxmaid.RoleEdge] = boxmaid.Style{Color: "#93a8c4"}
	theme.Styles[boxmaid.RoleLifeline] = boxmaid.Style{Color: "#93a8c4"}
	for row, cells := range e.picture.Cells {
		for col, cell := range cells {
			if cell.S == "" || cell.S == " " {
				continue
			}
			style := theme.CellStyle(cell)
			color := style.Color
			if !strings.HasPrefix(color, "#") {
				color = foreground
			}
			weight := "400"
			if style.Bold {
				weight = "600"
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" font-size="17" font-weight="%s" fill="%s" textLength="%d" lengthAdjust="spacingAndGlyphs">%s</text>`+"\n",
				x+28+col*cellWidth, y+80+row*lineHeight, weight, color, boxmaid.StringWidth(cell.S)*cellWidth, html.EscapeString(cell.S))
		}
	}
}

func hero(e example) []byte {
	height := 334 + panelHeight(e)
	var b strings.Builder
	start(&b, height, "boxmaid — Mermaid diagrams for the terminal", "Actual Boxmaid output: Mermaid source flows through native layout to terminal text. Go and Rust libraries with Bubble Tea and Ratatui integration.")
	text(&b, 52, 65, 16, "#5eead4", "GO + RUST  /  TERMINAL-NATIVE DIAGRAMS")
	text(&b, 48, 155, 80, foreground, "boxmaid")
	text(&b, 52, 207, 24, "#b6c6db", "Mermaid in. Terminal text out.")
	panel(&b, e, 48, 250, 1104)
	text(&b, 52, height-32, 16, "#9aadc5", "22 diagram families  ·  Unicode + ASCII  ·  Bubble Tea + Ratatui")
	b.WriteString("</g>\n</svg>\n")
	return []byte(b.String())
}

func gallery(examples []example) []byte {
	height := 0
	for _, e := range examples {
		height = max(height, panelHeight(e))
	}
	height += 164
	var b strings.Builder
	start(&b, height, "Boxmaid sequence and class diagrams", "Actual renderer output: a CLI/renderer sequence diagram, and a class diagram showing Renderer producing Picture. Colours use a higher-contrast Nord-based palette.")
	text(&b, 52, 57, 28, foreground, "One renderer. More than flowcharts.")
	text(&b, 52, 90, 16, "#9aadc5", "Real output, generated from the examples in this repository.")
	for i, e := range examples {
		panel(&b, e, 48+i*564, 120, 540)
	}
	b.WriteString("</g>\n</svg>\n")
	return []byte(b.String())
}

func generate(check bool) error {
	var examples []example
	for _, path := range []string{"docs/examples/pipeline.mmd", "docs/examples/sequence.mmd", "docs/examples/classes.mmd"} {
		e, err := load(path)
		if err != nil {
			return err
		}
		examples = append(examples, e)
	}
	for _, asset := range []struct {
		path string
		data []byte
	}{
		{"docs/assets/hero.svg", hero(examples[0])},
		{"docs/assets/gallery.svg", gallery(examples[1:])},
	} {
		if check {
			got, err := os.ReadFile(asset.path)
			if err != nil {
				return err
			}
			if !bytes.Equal(got, asset.data) {
				return fmt.Errorf("%s is stale; run go run ./internal/artwork", asset.path)
			}
		} else if err := os.WriteFile(asset.path, asset.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	check := flag.Bool("check", false, "fail if the committed artwork differs from current output")
	flag.Parse()
	if err := generate(*check); err != nil {
		fmt.Fprintln(os.Stderr, "artwork:", err)
		os.Exit(1)
	}
}
