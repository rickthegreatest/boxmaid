package renderer

import "strings"

type markdownFence struct {
	indent, marker, info string
}

func parseFence(line string) (markdownFence, bool) {
	line = strings.TrimRight(line, "\r\n")
	rest := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(rest)]
	if len(rest) < 3 || rest[0] != '`' && rest[0] != '~' {
		return markdownFence{}, false
	}
	n := 1
	for n < len(rest) && rest[n] == rest[0] {
		n++
	}
	if n < 3 || rest[0] == '`' && strings.ContainsRune(rest[n:], '`') {
		return markdownFence{}, false
	}
	return markdownFence{indent, rest[:n], strings.TrimSpace(rest[n:])}, true
}

// ReplaceMermaid replaces fenced mermaid blocks with fenced text drawings.
// It preserves other fenced blocks, unsupported diagrams, unclosed fences,
// and the document's line endings. It is a fenced-block transform, not a
// full Markdown parser (for example, blockquote-prefixed fences are left alone).
func ReplaceMermaid(md string, opt Options) string {
	lines := strings.SplitAfter(md, "\n")
	var out strings.Builder
	for i := 0; i < len(lines); {
		open, ok := parseFence(lines[i])
		if !ok {
			out.WriteString(lines[i])
			i++
			continue
		}
		end := i + 1
		for ; end < len(lines); end++ {
			close, ok := parseFence(lines[end])
			if ok && close.info == "" && close.marker[0] == open.marker[0] && len(close.marker) >= len(open.marker) {
				break
			}
		}
		if end == len(lines) {
			out.WriteString(strings.Join(lines[i:], ""))
			break
		}
		info := strings.Fields(open.info)
		var pic *Picture
		if len(info) > 0 && info[0] == "mermaid" {
			var src strings.Builder
			for _, line := range lines[i+1 : end] {
				src.WriteString(strings.TrimPrefix(line, open.indent))
			}
			pic, _ = Render(src.String(), opt)
		}
		if pic == nil {
			out.WriteString(strings.Join(lines[i:end+1], ""))
		} else {
			eol := "\n"
			if strings.HasSuffix(lines[i], "\r\n") {
				eol = "\r\n"
			}
			out.WriteString(open.indent + open.marker + "text" + eol)
			if pic.Title != "" {
				out.WriteString(open.indent + pic.Title + eol + eol)
			}
			for _, line := range pic.Lines() {
				out.WriteString(open.indent + line + eol)
			}
			out.WriteString(lines[end])
		}
		i = end + 1
	}
	return out.String()
}
