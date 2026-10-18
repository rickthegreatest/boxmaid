package boxmaid

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	brTag     = regexp.MustCompile(`(?i)<br\s*/?>`)
	anyTag    = regexp.MustCompile(`</?[a-zA-Z][^<>]*>`)
	entity    = regexp.MustCompile(`#([a-zA-Z]+|\d+);`)
	faIcon    = regexp.MustCompile(`\b(fa[bsrl]?|fa-solid|fa-regular):fa-[a-z0-9-]+\s*`)
	mdStrong  = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	mdEmph    = regexp.MustCompile(`(^|[^*\w])\*([^*\s][^*]*?)\*|(^|[^_\w])_([^_\s][^_]*?)_`)
	spaceRuns = regexp.MustCompile(`[ \t]+`)
)

// label turns a Mermaid label into plain lines: <br> and "\n" break lines,
// Mermaid's #quot; style entities and HTML entities decode, tags, icons and
// Markdown emphasis drop away.
func label(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '`' && s[len(s)-1] == '`') {
		s = strings.TrimSpace(s[1 : len(s)-1])
		s = strings.Trim(s, "`")
	}
	s = brTag.ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = anyTag.ReplaceAllString(s, "")
	// Mermaid's own entity syntax (#quot; #35;), then HTML's (&quot;).
	s = entity.ReplaceAllStringFunc(s, func(e string) string {
		name := e[1 : len(e)-1]
		if name != "" && name[0] >= '0' && name[0] <= '9' {
			name = "#" + name
		}
		if d, ok := decodeEntity(name); ok {
			return d
		}
		return e
	})
	s = unescapeHTML(s)
	s = faIcon.ReplaceAllString(s, "")
	s = mdStrong.ReplaceAllString(s, "$1$2")
	s = mdEmph.ReplaceAllString(s, "$1$2$3$4")
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(spaceRuns.ReplaceAllString(l, " "))
		out = append(out, l)
	}
	for len(out) > 1 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// wrap breaks lines longer than max columns at spaces (or anywhere, for
// words longer than max).
func wrap(lines []string, max int) []string {
	if max <= 0 {
		return lines
	}
	var out []string
	for _, l := range lines {
		if width(l) <= max {
			out = append(out, l)
			continue
		}
		cur := ""
		for _, w := range strings.Fields(l) {
			// Work with the same visible graphemes the canvas will draw. This
			// keeps a split's byte offset valid when leading zero-width text drops.
			gs, _ := graphemes(w)
			w = strings.Join(gs, "")
			for width(w) > max {
				head := breakWord(w, max)
				if cur != "" {
					out = append(out, cur)
					cur = ""
				}
				out = append(out, head)
				w = w[len(head):]
			}
			switch {
			case cur == "":
				cur = w
			case width(cur)+1+width(w) <= max:
				cur += " " + w
			default:
				out = append(out, cur)
				cur = w
			}
		}
		out = append(out, cur)
	}
	return out
}

func width(s string) int { return StringWidth(s) }

func maxWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, width(l))
	}
	return w
}

// breakWord is the first piece of a word too long for a line: up to a
// natural break (after _ - / . , or between a lower and an upper case
// letter) when there is one in the second half, else exactly max columns.
func breakWord(w string, max int) string {
	gs, widths := graphemes(w)
	if len(gs) == 0 {
		return w
	}
	n, used := 0, 0
	for n < len(gs) && used+widths[n] <= max {
		used += widths[n]
		n++
	}
	if n == 0 { // a wide grapheme in one column: take it anyway to make progress
		return gs[0]
	}
	for i := n - 1; i >= n/2; i-- {
		r, _ := utf8.DecodeRuneInString(gs[i])
		next := rune(0)
		if i+1 < len(gs) {
			next, _ = utf8.DecodeRuneInString(gs[i+1])
		}
		if strings.ContainsRune("_-/.,:;|", r) || unicode.IsLower(r) && unicode.IsUpper(next) {
			return strings.Join(gs[:i+1], "")
		}
	}
	return strings.Join(gs[:n], "")
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
