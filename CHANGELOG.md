# Changelog

## 0.1.0 — unreleased

First release, of the Go module (`github.com/boxmaid/boxmaid`, plus `/tui` for Bubble Tea) and the Rust crate (`boxmaid`, with an optional `ratatui` feature).

- Draws 22 Mermaid diagram types: flowchart, sequence, class, state, ER, C4, gantt, pie, journey, timeline, mindmap, git graph, quadrant, XY chart, kanban, packet, sankey, radar, treemap, requirement, architecture and block.
- Uses a layered layout for graph diagrams, with nested subgraphs, ports, self loops, edge labels and multiplicities.
- Fits the output to a width by wrapping labels and switching direction.
- Has a pure-ASCII mode, Unicode-aware widths, and roles per cell for theming.
- Comes with built-in ANSI themes: default, mono, dracula and nord.
- Helpers: `ReplaceMermaid` (render the mermaid blocks of a Markdown document) and `LiveURL` (link to mermaid.live).
- Includes a `boxmaid` command line tool in both languages.
