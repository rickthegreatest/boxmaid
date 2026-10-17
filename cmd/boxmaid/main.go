// Command boxmaid draws Mermaid diagrams in the terminal.
//
//	boxmaid diagram.mmd           draw a diagram
//	boxmaid < diagram.mmd         from standard input
//	boxmaid -md README.md         draw every ```mermaid block of a Markdown file
//	boxmaid -live diagram.mmd     print a mermaid.live link instead
//	boxmaid -list                 the diagram types boxmaid draws
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/boxmaid/boxmaid"
)

func main() {
	width := flag.Int("w", 0, "width to fit in (default: the terminal's)")
	ascii := flag.Bool("ascii", false, "plain ASCII only")
	colorMode := flag.String("color", "auto", "colour: auto, always or never")
	themeName := flag.String("theme", "default", "colour theme: "+strings.Join(themeNames(), ", "))
	md := flag.Bool("md", false, "the input is Markdown: draw its ```mermaid blocks")
	live := flag.Bool("live", false, "print a mermaid.live link instead of drawing")
	list := flag.Bool("list", false, "list the diagram types boxmaid draws")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: boxmaid [flags] [file …]   (reads standard input without files)")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *list {
		fmt.Println(strings.Join(boxmaid.Kinds(), "\n"))
		return
	}
	theme, ok := boxmaid.Themes[*themeName]
	if !ok {
		fail(fmt.Errorf("unknown theme %q", *themeName))
	}
	tty := isTerminal(os.Stdout)
	if *width == 0 && tty {
		*width = terminalWidth(os.Stdout)
	}
	color := *colorMode == "always" || *colorMode == "auto" && tty && os.Getenv("NO_COLOR") == ""
	opt := boxmaid.Options{Width: *width, ASCII: *ascii}
	inputs := flag.Args()
	if len(inputs) == 0 {
		inputs = []string{"-"}
	}
	status := 0
	for _, in := range inputs {
		src, err := read(in)
		if err != nil {
			fail(err)
		}
		switch {
		case *live:
			fmt.Println(boxmaid.LiveURL(src))
		case *md:
			fmt.Print(boxmaid.ReplaceMermaid(src, opt))
		default:
			p, err := boxmaid.Render(src, opt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", in, err)
				status = 1
				continue
			}
			if p.Title != "" {
				fmt.Println(p.Title)
				fmt.Println()
			}
			if color {
				fmt.Println(p.ANSI(theme))
			} else {
				fmt.Println(p)
			}
		}
	}
	os.Exit(status)
}

func read(name string) (string, error) {
	if name == "-" {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	b, err := os.ReadFile(name)
	return string(b), err
}

func themeNames() []string {
	var out []string
	for n := range boxmaid.Themes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "boxmaid:", err)
	os.Exit(2)
}
