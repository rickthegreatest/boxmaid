// Command boxmaid renders Mermaid as terminal text. Run boxmaid -help for usage.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/rickthegreatest/boxmaid"
)

// Overridden by release builds with -ldflags '-X main.version=…'.
var version = "0.1.1"

type terminal struct {
	input, output bool
	width         int
	noColor       bool
}

func main() {
	// Let writes return EPIPE so early-closing consumers are handled by run,
	// rather than the Go runtime terminating stdout writes with SIGPIPE.
	signal.Ignore(syscall.SIGPIPE)
	t := terminal{isTerminal(os.Stdin), isTerminal(os.Stdout), terminalWidth(os.Stdout), os.Getenv("NO_COLOR") != ""}
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, t))
}

// run keeps process exits and terminal discovery outside the command logic.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, term terminal) int {
	fs := flag.NewFlagSet("boxmaid", flag.ContinueOnError)
	fs.SetOutput(stderr)
	width := fs.Int("w", 0, "column budget; 0 uses terminal width, or unlimited when piped")
	ascii := fs.Bool("ascii", false, "draw with plain ASCII characters")
	colorMode := fs.String("color", "auto", "colour: auto, always or never")
	themeName := fs.String("theme", "default", "colour theme: "+strings.Join(themeNames(), ", "))
	md := fs.Bool("md", false, "replace fenced Mermaid blocks in Markdown with plain text")
	live := fs.Bool("live", false, "print a mermaid.live URL; does not open a browser")
	list := fs.Bool("list", false, "list supported diagram headers")
	showVersion := fs.Bool("version", false, "print the version")
	requireFit := fs.Bool("require-fit", false, "exit 3 without drawing if the diagram exceeds the column budget")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: boxmaid [flags] [file …]\n\nRead stdin when no files are given; use - for stdin or -- before filenames starting with -.\nPlace flags before filenames. Width fitting is best-effort unless -require-fit is set.\n\n")
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\nExamples:\n  boxmaid diagram.mmd\n  boxmaid -w 80 -color never diagram.mmd\n  boxmaid -md README.md\n\nExit codes: 0 success, 1 render/output error, 2 usage/input error, 3 cannot fit.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	usageError := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "boxmaid: "+format+"\n", a...)
		return 2
	}
	if *width < 0 {
		return usageError("-w must be non-negative")
	}
	if *colorMode != "auto" && *colorMode != "always" && *colorMode != "never" {
		return usageError("unknown colour mode %q; use auto, always or never", *colorMode)
	}
	theme, ok := boxmaid.Themes[*themeName]
	if !ok {
		return usageError("unknown theme %q; use %s", *themeName, strings.Join(themeNames(), ", "))
	}
	if *md && *live || *requireFit && (*md || *live) {
		return usageError("-md, -live and -require-fit cannot be combined")
	}
	write := func(s string) int {
		if _, err := io.WriteString(stdout, s); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return 0 // A downstream reader closing early is normal in pipelines.
			}
			fmt.Fprintln(stderr, "boxmaid:", err)
			return 1
		}
		return 0
	}
	if *list {
		return write(strings.Join(boxmaid.Kinds(), "\n") + "\n")
	}
	if *showVersion {
		return write("boxmaid " + version + "\n")
	}
	if *width == 0 && term.output {
		*width = term.width
	}
	if *requireFit && *width <= 0 {
		return usageError("-require-fit needs -w or a terminal with a known width")
	}
	inputs := fs.Args()
	if len(inputs) == 0 {
		if term.input {
			return usageError("provide a file or pipe Mermaid source; use -help for examples")
		}
		inputs = []string{"-"}
	}
	color := *colorMode == "always" || *colorMode == "auto" && term.output && !term.noColor
	opt := boxmaid.Options{Width: *width, ASCII: *ascii}
	status := 0
	for _, input := range inputs {
		src, err := read(input, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "boxmaid: %s: %v\n", input, err)
			status = max(status, 2)
			continue
		}
		var output string
		switch {
		case *live:
			output = boxmaid.LiveURL(src) + "\n"
		case *md:
			output = boxmaid.ReplaceMermaid(src, opt)
		default:
			p, err := boxmaid.Render(src, opt)
			if err != nil {
				fmt.Fprintf(stderr, "boxmaid: %s: %v\n", input, err)
				status = max(status, 1)
				continue
			}
			if opt.Width > 0 && p.W > opt.Width {
				fmt.Fprintf(stderr, "boxmaid: %s: diagram is %d columns; budget is %d\n", input, p.W, opt.Width)
				if *requireFit {
					status = 3
					continue
				}
			}
			if p.Title != "" {
				output = p.Title + "\n\n"
			}
			if color {
				output += p.ANSI(theme) + "\n"
			} else {
				output += p.String() + "\n"
			}
		}
		// Stop on write failure, including a closed pipe, rather than reading more inputs.
		if _, err := io.WriteString(stdout, output); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return status
			}
			fmt.Fprintln(stderr, "boxmaid:", err)
			return max(status, 1)
		}
	}
	return status
}

func read(name string, stdin io.Reader) (string, error) {
	var b []byte
	var err error
	if name == "-" {
		b, err = io.ReadAll(stdin)
	} else {
		b, err = os.ReadFile(name)
	}
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", errors.New("input is not valid UTF-8")
	}
	return string(b), nil
}

func themeNames() []string {
	var out []string
	for n := range boxmaid.Themes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
