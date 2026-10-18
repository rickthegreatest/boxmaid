# How is Boxmaid structured?

Boxmaid uses concrete data models and composition. Diagram parsers share rendering machinery where their geometry is alike; charts and timelines have specialized layout where forcing them into a graph would lose clarity.

## What is the rendering pipeline?

```text
Mermaid source
  → clean front matter/directives; detect header
  → diagram-specific parser
  → graph layout OR specialized chart/timeline layout
  → shared canvas
  → Picture (semantic cell grid)
  → plain text / ANSI / Bubble Tea / Ratatui
```

The Go entry point is `Render` in `boxmaid.go`; Rust uses `render` in `rust/src/lib.rs`. Both register accepted headers and dispatch to a renderer. Rendering returns a picture and source title; titles remain metadata rather than being inserted into the cell grid.

## Where does each responsibility live?

| Responsibility | Go | Rust |
|---|---|---|
| Public API, dispatch, source cleanup | `boxmaid.go` | `src/lib.rs` |
| Graph nodes, edges, cluster membership | `graph.go` | `src/graph.rs` |
| Graph layout, ranking, ordering, placement, routing | `layout.go` | `src/layout.rs` |
| Graph drawing and width-fit attempts | `draw.go`, `canvas.go` | `src/draw.rs`, `src/canvas.rs` |
| Diagram syntax and specialized layout | `flowchart.go`, `sequence.go`, `charts.go`, etc. | Corresponding `src/*.rs` |
| Labels, graphemes, widths, ASCII | `text.go`, `width.go`, `asciitext.go` | Corresponding `src/*.rs` |
| Portable values | `dates.go`, `entities.go` | `src/dates.rs`, `src/entities.rs` |
| Theme/style resolution, ANSI | `ansi.go` | `src/ansi.rs` |
| Fenced Markdown transformation | `markdown.go` | `src/markdown.rs` |
| CLI and terminal discovery | `cmd/boxmaid/` | `src/bin/boxmaid.rs` |
| UI adapters | Separate `tui/` module | Feature-gated `src/widget.rs` |
| Documentation artwork | `internal/artwork/` | Uses the Go public API |

Graph-family renderers include flowchart, state, class, ER, C4, requirement, and architecture. Their parsers populate the same graph representation. Sequence diagrams use a participant/timeline model; charts use their own scale and series logic. All ultimately draw into the canvas.

## How does graph layout work?

The layered layout operates along two axes: flow and cross-flow. Its explicit passes collect edges, break cycles for ranking, create boundary ports, lay out child clusters, reserve attachment space, size nodes, split long edges into chains, reduce crossings, place items, assign routing tracks, and compute bounds.

Nested clusters are laid out recursively and treated as blocks by their parent. Boundary ports connect internal and external routes. Node membership belongs in the graph model, not in the layout passes. `moveNode` / `move_node` updates both the node and its owning cluster.

Width fitting tries label-wrap budgets and alternate directions. Graph fitting intentionally reparses for each attempt because parsing/layout state is mutable. Avoid reusing mutated state between attempts without making that ownership explicit. The first fitting attempt wins; otherwise the narrowest result is returned.

## Why is there no class hierarchy?

Neither Go nor Rust uses implementation inheritance. Reuse comes from graph/canvas composition, small helpers, renderer function dispatch, and native interfaces/traits at output boundaries (`fmt.Stringer`, `Display`, `Widget`). Adding an abstract diagram framework would add indirection without unifying the different geometry.

`Theme.CellStyle` / `Theme::cell_style` centralizes role styling, source-colour precedence, and emphasis. ANSI and custom output adapters can share this contract instead of recreating precedence rules.

## Which invariants must changes preserve?

1. **Deterministic text.** Source declaration order is retained; unordered maps must not affect placement or names.
2. **Consistent membership.** A node belongs to exactly one cluster; cluster parenting must remain acyclic.
3. **Cell-aware text.** Labels are wrapped/truncated on grapheme and cell boundaries, not byte offsets. A wide character has a continuation cell.
4. **Presentation separation.** The core does not require a terminal, browser, TUI framework, or network. CLI terminal discovery stays in adapters.
5. **Explicit fitting limits.** A requested budget is not a guarantee; callers inspect dimensions or request strict CLI fitting.
6. **Cross-language behavior.** Rendering changes land in both implementations and match shared fixtures. Browser-link compression may differ while decoding to equivalent state.
7. **Tolerant parsing is visible in the contract.** Unknown statements may be skipped; header recognition is not syntax validation.

## What tests protect those boundaries?

- Unit/API tests check parsing, width fitting, Unicode, Markdown fence boundaries, and error behavior.
- Shared `.mmd` fixtures and Unicode/ASCII goldens protect output geometry and parity.
- CLI contract checks exercise status codes, flags, strict fitting, Markdown transformation, and multiple widths.
- Adapter tests check viewport bounds, option refresh, and Ratatui buffer rendering.
- Artwork regeneration checks ensure README images still match current renderer geometry.

The full commands are in [CONTRIBUTING.md](../CONTRIBUTING.md). Snapshot equality alone is not evidence of correct Mermaid semantics: fixes also need a targeted behavioral assertion or a reviewed, minimal fixture.
