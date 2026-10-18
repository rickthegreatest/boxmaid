package main

import (
	"bytes"
	"io"
	"strings"
	"syscall"
	"testing"
)

func TestCommandContract(t *testing.T) {
	for _, tc := range []struct {
		name            string
		args            []string
		term            terminal
		status          int
		out, diagnostic string
	}{
		{"stdin", nil, terminal{}, 0, "A", ""},
		{"help", []string{"-help"}, terminal{}, 0, "", "Usage:"},
		{"version", []string{"-version"}, terminal{}, 0, "boxmaid " + version, ""},
		{"negative width", []string{"-w", "-1"}, terminal{}, 2, "", "non-negative"},
		{"invalid colour", []string{"-color", "sometimes"}, terminal{}, 2, "", "unknown colour mode"},
		{"invalid theme", []string{"-theme", "missing"}, terminal{}, 2, "", "unknown theme"},
		{"conflicting modes", []string{"-md", "-live"}, terminal{}, 2, "", "cannot be combined"},
		{"fit requires budget", []string{"-require-fit"}, terminal{}, 2, "", "needs -w"},
		{"strict fit", []string{"-require-fit", "-w", "1"}, terminal{}, 3, "", "budget is 1"},
		{"best effort fit", []string{"-w", "1"}, terminal{}, 0, "A", "budget is 1"},
		{"interactive prompt", nil, terminal{input: true}, 2, "", "provide a file"},
		{"ascii", []string{"-ascii"}, terminal{}, 0, "+", ""},
		{"no colour", nil, terminal{output: true, width: 80, noColor: true}, 0, "A", ""},
		{"explicit colour", []string{"-color", "always"}, terminal{noColor: true}, 0, "\x1b[", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			status := run(tc.args, strings.NewReader("flowchart LR\nA-->B\n"), &out, &diagnostic, tc.term)
			if status != tc.status || !strings.Contains(out.String(), tc.out) || !strings.Contains(diagnostic.String(), tc.diagnostic) {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, out.String(), diagnostic.String())
			}
			if tc.out == "" && out.Len() != 0 || tc.diagnostic == "" && diagnostic.Len() != 0 {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", out.String(), diagnostic.String())
			}
			if tc.name == "no colour" && strings.Contains(out.String(), "\x1b[") {
				t.Fatal("NO_COLOR was ignored")
			}
		})
	}
}

type brokenPipe struct{}

func (brokenPipe) Write([]byte) (int, error) { return 0, syscall.EPIPE }

func TestClosedPipeIsNotAnError(t *testing.T) {
	if status := run(nil, strings.NewReader("graph LR\nA-->B"), brokenPipe{}, io.Discard, terminal{}); status != 0 {
		t.Fatalf("closed pipe: exit %d", status)
	}
}
