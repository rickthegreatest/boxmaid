package boxmaid

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func render(t *testing.T, src string, opt Options) string {
	t.Helper()
	p, err := Render(src, opt)
	if err != nil {
		t.Fatalf("render: %v\n%s", err, src)
	}
	return p.String()
}

func TestFlowchartSyntax(t *testing.T) {
	g, err := parseFlowchart(`flowchart LR
  A[Start] -->|go| B(Round) & C{Choice?}
  B -- yes --> D[(Store)]
  C -. maybe .-> E[[Sub]]
  C ==> F>Flag]
  A --- G((Circle)); G ~~~ H
  A <--> I
  J["label with (parens) and [brackets]"] --> K
  L@{ shape: cyl, label: "Queue" } --o M
  click A callback
  classDef hot fill:#ff5500,stroke:#333
  class B hot
  style K stroke:#00aa00`)
	if err != nil {
		t.Fatal(err)
	}
	if g.dir != "LR" {
		t.Errorf("dir = %s", g.dir)
	}
	shapes := map[string]shape{"A": shapeRect, "B": shapeRound, "C": shapeRhombus, "D": shapeCylinder, "E": shapeSubroutine, "F": shapeAsym, "G": shapeCircle, "L": shapeCylinder}
	for id, want := range shapes {
		if n := g.nodes[id]; n == nil || n.shape != want {
			t.Errorf("%s shape = %+v, want %d", id, n, want)
		}
	}
	if got := strings.Join(g.nodes["J"].label, " "); got != "label with (parens) and [brackets]" {
		t.Errorf("quoted label = %q", got)
	}
	if g.nodes["B"].color != "#ff5500" || g.nodes["K"].color != "#00aa00" {
		t.Errorf("colours: B=%q K=%q", g.nodes["B"].color, g.nodes["K"].color)
	}
	type want struct {
		from, to, label string
		style           lineStyle
		head, tail      head
	}
	wants := []want{
		{"A", "B", "go", lineSolid, headArrow, headNone},
		{"A", "C", "go", lineSolid, headArrow, headNone},
		{"B", "D", "yes", lineSolid, headArrow, headNone},
		{"C", "E", "maybe", lineDotted, headArrow, headNone},
		{"C", "F", "", lineThick, headArrow, headNone},
		{"A", "G", "", lineSolid, headNone, headNone},
		{"G", "H", "", lineInvisible, headNone, headNone},
		{"A", "I", "", lineSolid, headArrow, headArrow},
		{"J", "K", "", lineSolid, headArrow, headNone},
		{"L", "M", "", lineSolid, headCircle, headNone},
	}
	if len(g.edges) != len(wants) {
		t.Fatalf("%d edges, want %d", len(g.edges), len(wants))
	}
	for i, w := range wants {
		e := g.edges[i]
		if e.from != w.from || e.to != w.to || strings.Join(e.label, " ") != w.label || e.style != w.style || e.head != w.head || e.tail != w.tail {
			t.Errorf("edge %d = %+v, want %+v", i, *e, w)
		}
	}
}

func TestSubgraphs(t *testing.T) {
	g, err := parseFlowchart(`graph TD
  X --> A1
  subgraph one [First group]
    A1 --> A2
  end
  subgraph two
    direction LR
    B1
  end
  A2 --> two
  X`)
	if err != nil {
		t.Fatal(err)
	}
	if g.nodes["A1"].cluster.id != "one" || g.nodes["A1"].cluster.title != "First group" {
		t.Errorf("A1 belongs to %+v", g.nodes["A1"].cluster)
	}
	if g.nodes["X"].cluster != g.root {
		t.Error("X belongs at the top level")
	}
	if _, isNode := g.nodes["two"]; isNode {
		t.Error("an edge to a subgraph shouldn't create a node")
	}
	out := render(t, `graph TD
  X --> A1
  subgraph one [First group]
    A1 --> A2
  end
  A2 --> Y`, Options{})
	for _, s := range []string{"First group", "X", "A1", "A2", "Y", "▼"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q:\n%s", s, out)
		}
	}
}

// The drawing never lets a line run through a node's text.
func TestEdgesDontCrossText(t *testing.T) {
	out := render(t, `flowchart TD
  A[Request arrives] --> B{Valid?}
  B -- Yes --> C[Process]
  B -- No --> D[Reject]
  C --> E[(Store)]
  D --> E
  E --> A`, Options{})
	for _, word := range []string{"Request arrives", "Valid?", "Process", "Reject", "Store", "Yes", "No"} {
		if !strings.Contains(out, word) {
			t.Errorf("%q garbled:\n%s", word, out)
		}
	}
}

func TestFitsWidth(t *testing.T) {
	src := `flowchart LR
  A[Collect all the signals] --> B[Resolve the recommendation] --> C[Persist the decision] --> D[Publish the outcome event] --> E[Notify the agent]`
	wide, _ := Render(src, Options{})
	narrow, _ := Render(src, Options{Width: 80})
	if narrow.W > 80 || narrow.W >= wide.W {
		t.Fatalf("width 80: got %d (unlimited %d)", narrow.W, wide.W)
	}
}

func TestSequence(t *testing.T) {
	out := render(t, `sequenceDiagram
  autonumber
  actor U as Customer
  participant A as API
  U->>+A: POST /checkout
  alt ok
    A-->>U: 201
  else failed
    A--xU: 402
  end
  A->>A: retry
  Note over U,A: idempotent
  deactivate A`, Options{})
	for _, s := range []string{"Customer", "API", "① POST /checkout", "alt", "[ok]", "else", "② 201", "×", "retry", "idempotent", "►", "◄"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q:\n%s", s, out)
		}
	}
}

func TestStateAndC4AndPie(t *testing.T) {
	st := render(t, "stateDiagram-v2\n  [*] --> Idle\n  Idle --> Busy : job\n  state Busy {\n    [*] --> Working\n  }\n  Busy --> [*]", Options{})
	for _, s := range []string{"●", "◉", "Idle", "job", "Busy", "Working"} {
		if !strings.Contains(st, s) {
			t.Errorf("state: missing %q:\n%s", s, st)
		}
	}
	c4 := render(t, `C4Context
  Person(u, "User", "A customer")
  System_Boundary(b, "Shop") {
    System(s, "Shop API", "Takes orders")
  }
  Rel(u, s, "Uses", "HTTPS")`, Options{})
	for _, s := range []string{"User", "[Person]", "Shop API", "Takes orders", "Uses", "[HTTPS]", "Shop"} {
		if !strings.Contains(c4, s) {
			t.Errorf("c4: missing %q:\n%s", s, c4)
		}
	}
	pie := render(t, "pie title Pets\n  \"Dogs\" : 3\n  \"Cats\" : 1", Options{})
	if !strings.Contains(pie, "Dogs") || !strings.Contains(pie, "75.0%") {
		t.Errorf("pie:\n%s", pie)
	}
}

func TestASCII(t *testing.T) {
	out := render(t, "flowchart TD\n  A --> B{x?}\n  B -.->|no| C((c))", Options{ASCII: true})
	for _, r := range out {
		if r > 127 {
			t.Fatalf("non-ASCII %q in:\n%s", r, out)
		}
	}
}

func TestUnsupportedAndBroken(t *testing.T) {
	_, err := Render("zenuml\n  title Order\n  Alice->Bob: hi", Options{})
	var u *UnsupportedError
	if !errors.As(err, &u) || u.Kind != "zenuml" {
		t.Fatalf("zenuml: %v", err)
	}
	if _, err := Render("", Options{}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: %v", err)
	}
	// Nothing malformed may panic.
	for _, src := range []string{
		"flowchart TD\n  A -->", "flowchart\n  A[unclosed --> B", "graph LR\n  subgraph\n  end\n  end", "sequenceDiagram\n  end\n  else",
		"stateDiagram\n  }\n  [*] --> [*]", "C4Context\n  Rel(a, b)", "flowchart TD\n  A --> A\n  A --> A", "pie",
		"flowchart TD\n  subgraph s\n    a --> b\n  end\n  b --> s\n  s --> a",
	} {
		Render(src, Options{Width: 40})
	}
}

func TestFrontMatterAndDirectives(t *testing.T) {
	p, err := Render("---\ntitle: My flow\nconfig:\n  theme: dark\n---\n%%{init: {'theme':'dark'}}%%\nflowchart TD\n  %% a comment\n  A --> B", Options{})
	if err != nil || p.Title != "My flow" || !strings.Contains(p.String(), "A") {
		t.Fatalf("title %q err %v", p.Title, err)
	}
}

func TestLiveURL(t *testing.T) {
	u := LiveURL("flowchart TD\n  A --> B")
	enc := strings.TrimPrefix(u, "https://mermaid.live/view#pako:")
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(zr)
	var state struct{ Code string }
	if json.Unmarshal(b, &state) != nil || state.Code != "flowchart TD\n  A --> B" {
		t.Fatalf("state %s", b)
	}
}
