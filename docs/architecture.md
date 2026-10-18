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

The public Go entry point is `Render` in `boxmaid.go`. It delegates parsing and layout to `internal/renderer`, then adapts the final drawing to the public named types. Rust uses `render` in `rust/src/lib.rs`. Rendering returns a picture and source title; titles remain metadata rather than being inserted into the cell grid.

## Where does each responsibility live?

```text
boxmaid.go           public Go types and entry points
style.go             public Go themes and ANSI output
boxmaid_test.go      public contract, shared golden, and fuzz tests
cmd/boxmaid/         Go CLI and terminal discovery
internal/
  renderer/          Go parsers, layout, canvas, and implementation tests
  artwork/           reproducible README artwork
tui/                 separately versioned Bubble Tea adapter and example
rust/                Rust crate, CLI, Ratatui adapter, and tests
testdata/            shared Mermaid cases, expected drawings, and fuzz seeds
docs/                guides, examples, artwork, release notes, and website
scripts/             release and cross-language contract checks
```

The Go implementation stays one cohesive internal package because graph parsers share private graph/layout structures. This keeps the repository root focused on the public interface without creating artificial package boundaries between tightly coupled passes. Implementation tests live beside that code; public golden and fuzz tests remain at the root so corpus paths and regeneration commands stay stable.

| Responsibility | Go | Rust |
|---|---|---|
| Public API and named types | `boxmaid.go` | `rust/src/lib.rs` |
| Dispatch and source cleanup | `internal/renderer/renderer.go` | `rust/src/lib.rs` |
| Graph nodes, edges, cluster membership | `internal/renderer/graph.go` | `rust/src/graph.rs` |
| Graph layout, ranking, ordering, placement, routing | `internal/renderer/layout.go` | `rust/src/layout.rs` |
| Graph drawing and width-fit attempts | `internal/renderer/draw.go`, `canvas.go` | `rust/src/draw.rs`, `canvas.rs` |
| Diagram syntax and specialized layout | `internal/renderer/flowchart.go`, `sequence.go`, `charts.go`, etc. | Corresponding `rust/src/*.rs` |
| Labels, graphemes, widths, ASCII | `internal/renderer/text.go`, `width.go`, `asciitext.go` | Corresponding `rust/src/*.rs` |
| Portable values | `internal/renderer/dates.go`, `entities.go` | `rust/src/dates.rs`, `entities.rs` |
| Theme/style resolution, ANSI | `style.go` | `rust/src/ansi.rs` |
| Fenced Markdown transformation | `internal/renderer/markdown.go` | `rust/src/markdown.rs` |
| CLI and terminal discovery | `cmd/boxmaid/` | `rust/src/bin/boxmaid.rs` |
| UI adapters | Separate `tui/` module | Feature-gated `rust/src/widget.rs` |
| Documentation artwork | `internal/artwork/` | Uses the Go public API |

Graph-family renderers include flowchart, state, class, ER, C4, requirement, and architecture. Their parsers populate the same graph representation. Sequence diagrams use a participant/timeline model; charts use their own scale and series logic. All ultimately draw into the canvas.

## How is the Go API kept compatible?

Public `Picture`, `Cell`, `Options`, `Role`, `Theme`, and error types remain defined in the root package. Their import path, named type identities, fields, and method sets are retained. The public cell grid is built once, directly from the selected canvas after width fitting, with a single backing allocation. Unsuccessful layout attempts do not materialize extra cell grids. Plain-text formatting is shared between the public and internal representations; styling operates directly on the public grid.

Keeping public types at the root also keeps their methods visible in Go API documentation. The internal package is an implementation boundary, not an additional import that consumers must learn.

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
