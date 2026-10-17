package boxmaid

import (
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
