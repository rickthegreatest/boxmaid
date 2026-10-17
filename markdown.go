package boxmaid

import (
	"regexp"
	"strings"
)

var mermaidFence = regexp.MustCompile("(?ms)^([ \t]*)(`{3,}|~{3,})[ \t]*mermaid[^\n]*\n(.*?)^[ \t]*(`{3,}|~{3,})[ \t]*$")

// ReplaceMermaid draws the ```mermaid blocks of a Markdown document,
// replacing each with a ```text block holding the drawing, so any Markdown
// renderer (glamour, a pager, a chat message) shows the diagram. Blocks
// that can't be drawn are left as they are.
func ReplaceMermaid(md string, opt Options) string {
	return mermaidFence.ReplaceAllStringFunc(md, func(block string) string {
		m := mermaidFence.FindStringSubmatch(block)
		indent, fence, src := m[1], m[2], m[3]
		if !strings.HasPrefix(m[4], fence[:1]) || len(m[4]) < len(fence) {
			return block
		}
		p, err := Render(src, opt)
		if err != nil {
			return block
		}
		var b strings.Builder
		b.WriteString(indent + fence + "text\n")
		if p.Title != "" {
			b.WriteString(indent + p.Title + "\n\n")
		}
		for _, l := range p.Lines() {
			b.WriteString(indent + l + "\n")
		}
		b.WriteString(indent + fence)
		return b.String()
	})
}
