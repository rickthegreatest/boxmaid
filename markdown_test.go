package boxmaid

import (
	"strings"
	"testing"
)

func TestMarkdownFenceBoundaries(t *testing.T) {
	for _, md := range []string{
		"````markdown\n```mermaid\nflowchart LR\nA-->B\n```\n````\n",
		"```mermaid-example\nflowchart LR\nA-->B\n```\n",
		"```mermaid\nflowchart LR\nA-->B\n~~~\n",
		"````mermaid\nflowchart LR\nA-->B\n```\n",
		"```mermaid\nflowchart LR\nA-->B\n",
		"```mermaid\nzenuml\n```\n",
	} {
		if got := ReplaceMermaid(md, Options{}); got != md {
			t.Errorf("non-renderable block changed:\n%q\nwant:\n%q", got, md)
		}
	}
	md := "Intro\r\n\r\n  ~~~mermaid\r\n  flowchart LR\r\n  A-->B\r\n  ~~~~\r\nAfter\r\n"
	got := ReplaceMermaid(md, Options{})
	if !strings.Contains(got, "  ~~~text\r\n") || !strings.HasSuffix(got, "  ~~~~\r\nAfter\r\n") {
		t.Fatalf("indentation or closing fence lost: %q", got)
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings: %q", got)
	}
}
