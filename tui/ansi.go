package tui

import (
	"github.com/charmbracelet/x/ansi"
)

func dropANSI(s string, n int) string     { return ansi.TruncateLeft(s, n, "") }
func truncateANSI(s string, w int) string { return ansi.Truncate(s, w, "") }
