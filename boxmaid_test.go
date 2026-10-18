package boxmaid

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shared corpus: testdata/cases/*.mmd, drawn in Unicode at width 100
// and in ASCII at width 80, must match testdata/golden exactly. The Rust
// crate checks the same files, so the two implementations can't drift.
//
//	go test -run Golden -update   rewrites the golden files

var update = flag.Bool("update", false, "rewrite the golden files")

// goldenText is what a golden file holds for one rendering.
func goldenText(src string, opt Options) string {
	p, err := Render(src, opt)
	if err != nil {
		return "error: " + err.Error() + "\n"
	}
	return "kind: " + p.Kind + "\ntitle: " + p.Title + "\n\n" + p.String() + "\n"
}

func TestGolden(t *testing.T) {
	cases, _ := filepath.Glob("testdata/cases/*.mmd")
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range cases {
		name := strings.TrimSuffix(filepath.Base(c), ".mmd")
		src, err := os.ReadFile(c)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range []struct {
			suffix string
			opt    Options
		}{{".txt", Options{Width: 100}}, {".ascii.txt", Options{Width: 80, ASCII: true}}} {
			got := goldenText(string(src), v.opt)
			path := filepath.Join("testdata/golden", name+v.suffix)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: no golden file (run go test -update)", path)
				continue
			}
			if got != string(want) {
				t.Errorf("%s differs from its golden file:\n--- got\n%s\n--- want\n%s", path, got, want)
			}
		}
	}
}

// ASCII mode is pure ASCII.
func TestASCIIGoldensArePureASCII(t *testing.T) {
	files, _ := filepath.Glob("testdata/golden/*.ascii.txt")
	for _, f := range files {
		b, _ := os.ReadFile(f)
		for i, r := range string(b) {
			if r > 127 {
				t.Errorf("%s: %q at byte %d", f, r, i)
				break
			}
		}
	}
}

// Exercise errors through the public boundary, including the concrete type
// used by clients with errors.As and the sentinel values used with errors.Is.
func TestPublicErrors(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   error
	}{{"", ErrEmpty}, {"flowchart LR\n\xff", ErrInvalidUTF8}} {
		if _, err := Render(tc.source, Options{}); !errors.Is(err, tc.want) {
			t.Fatalf("Render(%q): %v, want %v", tc.source, err, tc.want)
		}
	}
	_, err := Render("zenuml", Options{})
	var unsupported *UnsupportedError
	if !errors.As(err, &unsupported) || unsupported.Kind != "zenuml" {
		t.Fatalf("unsupported error lost its public type: %v", err)
	}
}

func TestCellStylePrecedence(t *testing.T) {
	cell := Cell{Role: RoleNode, Color: "#123456", Bold: true}
	if got := NordTheme.CellStyle(cell); got != (Style{Color: "#123456", Bold: true}) {
		t.Fatalf("explicit colour and emphasis lost: %+v", got)
	}
	if got := DefaultTheme.CellStyle(cell); got.Color != "4" || !got.Bold {
		t.Fatalf("basic theme must ignore source true-colour but retain emphasis: %+v", got)
	}
}

func TestANSIRejectsInvalidPaletteNumbers(t *testing.T) {
	for _, color := range []string{"-1", "256", "999999", "not-a-colour"} {
		if got := sgr(Style{Color: color}, false); got != "" {
			t.Errorf("invalid palette entry %q produced %q", color, got)
		}
	}
}

func TestGitGraphBottomToTop(t *testing.T) {
	p, err := Render("gitGraph BT:\ncommit id: \"root\"\ncommit id: \"leaf\"", Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := p.String()
	if root, leaf := strings.Index(text, "root"), strings.Index(text, "leaf"); root < 0 || leaf < 0 || leaf >= root {
		t.Fatalf("bottom-to-top commits must run from the last commit up to the first:\n%s", text)
	}
}

// Fuzz the public entry point; corpus paths stay alongside the shared goldens.
func FuzzFlowchartRender(f *testing.F) {
	for _, body := range []string{
		"A-->B-->C",
		"A{Ready?}-->|yes|B\nB-->A",
		"subgraph s[Group]\nA-->B\nend\nB-->s",
		"A[界👩‍💻]-->B[" + strings.Repeat("x", 40) + "]",
		"A-->",
		"A:::café --> B",
		"A[\u200b" + strings.Repeat("界", 30) + "]",
	} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 256 {
			t.Skip()
		}
		pic, err := Render("flowchart LR\n"+body, Options{Width: 80})
		if err == nil && (pic.W < 0 || pic.H < 0 || len(pic.Cells) != pic.H) {
			t.Fatalf("invalid picture dimensions: %dx%d, %d rows", pic.W, pic.H, len(pic.Cells))
		}
	})
}
