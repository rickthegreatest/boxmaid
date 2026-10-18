# boxmaid

**Render Mermaid as terminal text in Rust.** Unicode box drawing or ASCII, semantic cell styling, width-aware layout, and an optional Ratatui widget. No browser or JavaScript runtime required.

## How do I render a diagram?

```rust
let pic = boxmaid::render("flowchart LR\n  A --> B", &boxmaid::Options { width: 80, ascii: false })?;
println!("{pic}");
println!("{}", pic.ansi(&boxmaid::Theme::nord()));
```

`Picture` contains dimensions, title metadata, and a grid of `Cell`s. Each cell has a semantic `Role`, optional source colour, and emphasis. `Theme::cell_style` resolves those properties for custom adapters.

## How do I use it in Ratatui?

Enable the `ratatui` feature and render `&Picture` as a widget, or use `Diagram::new(&pic).theme(...).scroll(x, y)`. Repeat rendering when the column budget changes. See the [viewer example](https://github.com/rickthegreatest/boxmaid/blob/main/rust/examples/ratatui_viewer.rs).

## Can I use a CLI or transform Markdown?

Install the Rust CLI from the release with `cargo install --git https://github.com/rickthegreatest/boxmaid --tag v0.1.0 --locked boxmaid`. Use `boxmaid -w 80 -color never -require-fit diagram.mmd` for predictable output. `replace_mermaid` transforms fenced Mermaid blocks into text; `live_url` constructs a browser fallback URL without making a network request.

## What are the compatibility limits?

Boxmaid supports 22 diagram families with selective syntax coverage. Unknown statements within supported diagrams are generally skipped. Some charts become terminal-friendly bars/lists. Fitting is best-effort: compare `pic.w` with the requested width, or use CLI `-require-fit`.

The crate requires Rust 1.88+ and depends on `regex`; Ratatui is optional. The [Go implementation](https://github.com/rickthegreatest/boxmaid) is checked against the same shared text fixtures.

## Where are the full docs?

[README](https://github.com/rickthegreatest/boxmaid#readme) · [Syntax coverage](https://github.com/rickthegreatest/boxmaid/blob/main/docs/syntax.md) · [Agent guide](https://github.com/rickthegreatest/boxmaid/blob/main/docs/agents.md) · [Architecture](https://github.com/rickthegreatest/boxmaid/blob/main/docs/architecture.md)

## What is the license?

MIT.
