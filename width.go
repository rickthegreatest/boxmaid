package boxmaid

import "unicode/utf8"

// Terminal cell widths, without dependencies. The Go and Rust
// implementations carry the same tables so they measure text identically:
// a character is 2 cells when it's East Asian wide or an emoji, 0 when it
// combines with the one before it, else 1. A grapheme is a character with
// the zero-width ones after it, ZWJ sequences and flag pairs joined.

// wideRanges are the inclusive ranges of 2-cell characters, sorted.
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, {0x231A, 0x231B}, {0x2329, 0x232A}, {0x23E9, 0x23EC}, {0x23F0, 0x23F0}, {0x23F3, 0x23F3},
	{0x25FD, 0x25FE}, {0x2614, 0x2615}, {0x2648, 0x2653}, {0x267F, 0x267F}, {0x2693, 0x2693}, {0x26A1, 0x26A1},
	{0x26AA, 0x26AB}, {0x26BD, 0x26BE}, {0x26C4, 0x26C5}, {0x26CE, 0x26CE}, {0x26D4, 0x26D4}, {0x26EA, 0x26EA},
	{0x26F2, 0x26F3}, {0x26F5, 0x26F5}, {0x26FA, 0x26FA}, {0x26FD, 0x26FD}, {0x2705, 0x2705}, {0x270A, 0x270B},
	{0x2728, 0x2728}, {0x274C, 0x274C}, {0x274E, 0x274E}, {0x2753, 0x2755}, {0x2757, 0x2757}, {0x2795, 0x2797},
	{0x27B0, 0x27B0}, {0x27BF, 0x27BF}, {0x2B1B, 0x2B1C}, {0x2B50, 0x2B50}, {0x2B55, 0x2B55}, {0x2E80, 0x303E},
	{0x3041, 0x33FF}, {0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF}, {0xA960, 0xA97F}, {0xAC00, 0xD7A3},
	{0xF900, 0xFAFF}, {0xFE10, 0xFE19}, {0xFE30, 0xFE6F}, {0xFF00, 0xFF60}, {0xFFE0, 0xFFE6}, {0x1F004, 0x1F004},
	{0x1F0CF, 0x1F0CF}, {0x1F18E, 0x1F18E}, {0x1F191, 0x1F19A}, {0x1F200, 0x1F202}, {0x1F210, 0x1F23B},
	{0x1F240, 0x1F248}, {0x1F250, 0x1F251}, {0x1F260, 0x1F265}, {0x1F300, 0x1F320}, {0x1F32D, 0x1F335},
	{0x1F337, 0x1F37C}, {0x1F37E, 0x1F393}, {0x1F3A0, 0x1F3CA}, {0x1F3CF, 0x1F3D3}, {0x1F3E0, 0x1F3F0},
	{0x1F3F4, 0x1F3F4}, {0x1F3F8, 0x1F43E}, {0x1F440, 0x1F440}, {0x1F442, 0x1F4FC}, {0x1F4FF, 0x1F53D},
	{0x1F54B, 0x1F54E}, {0x1F550, 0x1F567}, {0x1F57A, 0x1F57A}, {0x1F595, 0x1F596}, {0x1F5A4, 0x1F5A4},
	{0x1F5FB, 0x1F64F}, {0x1F680, 0x1F6C5}, {0x1F6CC, 0x1F6CC}, {0x1F6D0, 0x1F6D2}, {0x1F6D5, 0x1F6D7},
	{0x1F6DC, 0x1F6DF}, {0x1F6EB, 0x1F6EC}, {0x1F6F4, 0x1F6FC}, {0x1F7E0, 0x1F7EB}, {0x1F7F0, 0x1F7F0},
	{0x1F90C, 0x1F93A}, {0x1F93C, 0x1F945}, {0x1F947, 0x1F9FF}, {0x1FA70, 0x1FAFF}, {0x20000, 0x2FFFD},
	{0x30000, 0x3FFFD},
}

// zeroRanges are the inclusive ranges of characters that take no cell of
// their own (combining marks, joiners, variation selectors, skin tones).
var zeroRanges = [][2]rune{
	{0x0300, 0x036F}, {0x0483, 0x0489}, {0x0591, 0x05BD}, {0x0610, 0x061A}, {0x064B, 0x065F}, {0x0E31, 0x0E31},
	{0x0E34, 0x0E3A}, {0x0E47, 0x0E4E}, {0x1AB0, 0x1AFF}, {0x1DC0, 0x1DFF}, {0x200B, 0x200F}, {0x2028, 0x202E},
	{0x2060, 0x2064}, {0x20D0, 0x20FF}, {0xFE00, 0xFE0F}, {0xFE20, 0xFE2F}, {0x1F3FB, 0x1F3FF}, {0xE0020, 0xE007F},
	{0xE0100, 0xE01EF},
}

const zwj = 0x200D

func inRanges(r rune, ranges [][2]rune) bool {
	lo, hi := 0, len(ranges)-1
	for lo <= hi {
		m := (lo + hi) / 2
		switch {
		case r < ranges[m][0]:
			hi = m - 1
		case r > ranges[m][1]:
			lo = m + 1
		default:
			return true
		}
	}
	return false
}

// RuneWidth is how many terminal cells r takes: 0, 1 or 2.
func RuneWidth(r rune) int {
	switch {
	case r == 0 || r == zwj || inRanges(r, zeroRanges):
		return 0
	case r < 0x20 || r == 0x7F:
		return 0
	case inRanges(r, wideRanges):
		return 2
	}
	return 1
}

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

// graphemes splits s into what occupies terminal cells, each with its
// width (1 or 2); zero-width text is dropped.
func graphemes(s string) (gs []string, ws []int) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		start := i
		i += size
		w := RuneWidth(r)
		// Flags: two regional indicators make one 2-cell grapheme.
		if isRegionalIndicator(r) && i < len(s) {
			if r2, s2 := utf8.DecodeRuneInString(s[i:]); isRegionalIndicator(r2) {
				i += s2
				w = 2
			}
		}
		// Zero-width followers belong to this grapheme; a ZWJ also takes the
		// character after it.
		for i < len(s) {
			n, ns := utf8.DecodeRuneInString(s[i:])
			if n == zwj {
				i += ns
				if i < len(s) {
					_, s3 := utf8.DecodeRuneInString(s[i:])
					i += s3
				}
				continue
			}
			if RuneWidth(n) != 0 || n < 0x20 {
				break
			}
			i += ns
		}
		if w == 0 {
			continue
		}
		gs = append(gs, s[start:i])
		ws = append(ws, w)
	}
	return gs, ws
}

// StringWidth is how many terminal cells s takes.
func StringWidth(s string) int {
	_, ws := graphemes(s)
	n := 0
	for _, w := range ws {
		n += w
	}
	return n
}

// truncate keeps the start of s that fits in w cells.
func truncate(s string, w int) string {
	gs, ws := graphemes(s)
	out, used := "", 0
	for i, g := range gs {
		if used+ws[i] > w {
			break
		}
		out += g
		used += ws[i]
	}
	return out
}
