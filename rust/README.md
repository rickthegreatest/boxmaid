# boxmaid

**[Mermaid](https://mermaid.js.org) diagrams for the terminal.** boxmaid draws flowcharts, sequence, class, state, ER, C4, gantt, pie and 14 more diagram types with Unicode box-drawing characters (or plain ASCII). It lays them out so edges don't cross nodes, and fits them to the width you have.

```rust
let pic = boxmaid::render("flowchart LR\n  A --> B", &boxmaid::Options { width: 80, ascii: false })?;
println!("{pic}");                                     // plain text
println!("{}", pic.ansi(&boxmaid::Theme::default()));  // coloured
```

- Every cell carries a `Role` (node, edge, arrow, label…) so you can colour it with your own theme. Built-in themes: default, mono, dracula, nord.
- The `ratatui` feature makes `&Picture` (and the scrollable `Diagram`) a ratatui `Widget`.
- `replace_mermaid` draws every mermaid block of a Markdown document. `live_url` links to mermaid.live.
- The `boxmaid` binary draws `.mmd` files and Markdown in the terminal: `cargo install boxmaid`.
- Its only dependency is `regex`.

The Go module [`github.com/boxmaid/boxmaid`](https://github.com/boxmaid/boxmaid) draws the same input to the same bytes; both are tested against one shared set of golden files.

See the [project README](https://github.com/boxmaid/boxmaid#readme) for the full list of diagrams and the [syntax coverage](https://github.com/boxmaid/boxmaid/blob/main/docs/syntax.md).

License: MIT
