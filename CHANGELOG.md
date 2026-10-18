# Changelog

## Unreleased

- Add a runnable Bubble Tea service-runbook demo, terminal recordings, and an embedding walkthrough with a Ratatui capture.
- Move Go parsing, layout, canvas, and implementation tests into `internal/renderer/`, leaving the public API, styling, and contract tests at the repository root.
- Preserve public Go named types and method sets, and build the public cell grid directly from the selected canvas after width fitting.
- Update the architecture map and contributor/agent guidance for the new package layout.

## 0.1.1 — 2026-10-18

- Correct the Go module path and best-effort fitting description in Rust's generated API documentation.
- Build docs.rs documentation with all features, including the Ratatui widget.
- Link the published packages, showcase website, and launch discussion from the project documentation.
- Align Go CLI, TUI dependency, and Rust package versions for the patch release.

## 0.1.0 — 2026-10-18

First release of the Go module (`github.com/rickthegreatest/boxmaid`, plus `/tui` for Bubble Tea) and the Rust crate (`boxmaid`, with an optional `ratatui` feature).

- Draws 22 Mermaid diagram types: flowchart, sequence, class, state, ER, C4, gantt, pie, journey, timeline, mindmap, git graph, quadrant, XY chart, kanban, packet, sankey, radar, treemap, requirement, architecture and block.
- Uses a layered layout for graph diagrams, with nested subgraphs, ports, self loops, edge labels and multiplicities.
- Fits the output to a width by wrapping labels and switching direction.
- Has a pure-ASCII mode, Unicode-aware widths, and roles per cell for theming.
- Comes with built-in ANSI themes: default, mono, dracula and nord.
- Helpers: `ReplaceMermaid` (render the mermaid blocks of a Markdown document) and `LiveURL` (link to mermaid.live).
- Includes a `boxmaid` command line tool in both languages.
- CLI contract: validated flags, successful help/version commands, predictable stderr/exit codes, pipeline-friendly output, and `-require-fit` for strict column budgets.
- Compact linear flowcharts, cell-aware Unicode pie-label truncation, overflow-safe pie proportions, and consistent membership for redeclared architecture groups/services.
- Unicode-safe class-name parsing, grapheme-aware word splitting, and standard UTF-8 identifier decoding; malformed byte input returns an error instead of panicking.
- Markdown transformations preserve outer code examples, matching fence lengths/characters, indentation, and CRLF.
- Bubble Tea viewport sizing, immediate style/options updates, and stable-height views; viewer examples expose navigation and position.
- Human- and agent-oriented guides, question-led README, architecture map, and reproducible SVG artwork generated from actual output.
- Cross-platform/minimum-version CI, CLI parity checks, package verification, and tagged binary-release automation.
- Rust minimum version is 1.88, matching the locked TUI dependency graph rather than advertising an unsupported toolchain.
