package renderer

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWordWrappingKeepsGraphemesAndProgress(t *testing.T) {
	for _, word := range []string{
		"\u200b" + strings.Repeat("界", 30),
		strings.Repeat("x", 15) + "-\u0301" + strings.Repeat("y", 20),
		strings.Repeat("👩‍💻", 20),
	} {
		lines := wrap([]string{word}, 28)
		for _, line := range lines {
			if !utf8.ValidString(line) || StringWidth(line) > 28 {
				t.Fatalf("invalid wrapped line %q", line)
			}
		}
		gs, _ := graphemes(word)
		if strings.Join(lines, "") != strings.Join(gs, "") {
			t.Fatalf("visible content lost while splitting %q: %q", word, lines)
		}
	}
}

func TestRenderRejectsInvalidUTF8(t *testing.T) {
	if _, err := Render("flowchart LR\n\xff", Options{}); err != ErrInvalidUTF8 {
		t.Fatalf("invalid source: %v", err)
	}
}
