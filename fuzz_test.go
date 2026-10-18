package boxmaid

import (
	"strings"
	"testing"
)

// Fuzz the bounded graph parser/layout together: syntax recovery must not panic.
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
