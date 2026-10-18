package renderer

import (
	"strconv"
	"strings"
)

// HTML entities in labels. A fixed table (the Rust crate carries the same
// one) rather than the full HTML5 list, so both implementations decode
// identically: the named entities people write in diagrams, and every
// numeric one.

var entities = map[string]string{
	"amp": "&", "lt": "<", "gt": ">", "quot": "\"", "apos": "'", "nbsp": " ",
	"copy": "©", "reg": "®", "trade": "™", "hellip": "…", "mdash": "—", "ndash": "–",
	"laquo": "«", "raquo": "»", "lsquo": "‘", "rsquo": "’", "ldquo": "“", "rdquo": "”",
	"larr": "←", "rarr": "→", "uarr": "↑", "darr": "↓", "harr": "↔", "lArr": "⇐", "rArr": "⇒",
	"times": "×", "divide": "÷", "deg": "°", "plusmn": "±", "middot": "·", "bull": "•",
	"euro": "€", "pound": "£", "yen": "¥", "cent": "¢", "sect": "§", "para": "¶",
	"le": "≤", "ge": "≥", "ne": "≠", "infin": "∞", "check": "✓", "hearts": "♥",
	"num": "#", "colon": ":", "semi": ";", "lpar": "(", "rpar": ")", "lsqb": "[", "rsqb": "]",
	"lcub": "{", "rcub": "}", "vert": "|", "excl": "!", "quest": "?", "percnt": "%", "plus": "+",
	"equals": "=", "commat": "@", "dollar": "$", "ast": "*", "sol": "/", "bsol": "\\",
}

// decodeEntity is the text for an entity's name ("amp", "#35", "#x23"),
// and whether it is one.
func decodeEntity(name string) (string, bool) {
	if strings.HasPrefix(name, "#x") || strings.HasPrefix(name, "#X") {
		n, err := strconv.ParseUint(name[2:], 16, 32)
		if err != nil || n == 0 || n > 0x10FFFF {
			return "", false
		}
		return string(rune(n)), true
	}
	if strings.HasPrefix(name, "#") {
		n, err := strconv.ParseUint(name[1:], 10, 32)
		if err != nil || n == 0 || n > 0x10FFFF {
			return "", false
		}
		return string(rune(n)), true
	}
	s, ok := entities[name]
	return s, ok
}

// unescapeHTML decodes &name; entities.
func unescapeHTML(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i:]
		j := strings.IndexByte(s, ';')
		if j > 1 && j <= 12 {
			if d, ok := decodeEntity(s[1:j]); ok {
				b.WriteString(d)
				s = s[j+1:]
				continue
			}
		}
		b.WriteByte('&')
		s = s[1:]
	}
}

// formatNumber writes a number the same way in Go and Rust: whole numbers
// without decimals, others with up to two.
func formatNumber(v float64) string {
	if v == float64(int64(v)) && v < 1e15 && v > -1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
