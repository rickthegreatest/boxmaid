package boxmaid

import "testing"

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
