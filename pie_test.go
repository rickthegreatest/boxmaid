package boxmaid

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPieLabelsPreserveGraphemes(t *testing.T) {
	name := strings.Repeat("界", 20)
	got := render(t, "pie\n\""+name+"\": 1", Options{})
	if !utf8.ValidString(got) || strings.ContainsRune(got, utf8.RuneError) || !strings.Contains(got, strings.Repeat("界", 14)+"…") {
		t.Fatalf("label was not truncated at a cell boundary: %q", got)
	}
}

func TestPieFiniteValuesDoNotOverflow(t *testing.T) {
	large := strings.Repeat("9", 308)
	got := render(t, "pie\n\"A\": "+large+"\n\"B\": "+large, Options{})
	if strings.Count(got, "50.0%") != 2 {
		t.Fatalf("finite values overflowed the total: %s", got)
	}
}
