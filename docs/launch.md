# What is new in Boxmaid 0.1.0?

**Mermaid in. Terminal text out.** Boxmaid's first release brings Mermaid previews to CLIs, TUIs, documentation pipelines, and agent tools, with native Go and Rust implementations.

![Boxmaid draws Mermaid source through native layout into terminal text.](https://raw.githubusercontent.com/rickthegreatest/boxmaid/main/docs/assets/hero.svg)

## What can I do with it?

- Render 22 diagram families as Unicode box drawing or plain ASCII.
- Give a diagram a column budget, or use strict-fit mode when overflow must fail.
- Embed semantic cell output in a Go or Rust application.
- Add pannable diagrams to Bubble Tea, or a themed widget to Ratatui.
- Replace fenced Mermaid in Markdown with fenced text for terminal documentation.

Rendering is local. There is no browser or JavaScript runtime requirement. Syntax support is selective, with [accepted constructs and omissions documented](https://github.com/rickthegreatest/boxmaid/blob/main/docs/syntax.md).

## How do I try it?

```sh
go install github.com/rickthegreatest/boxmaid/cmd/boxmaid@v0.1.0
printf 'flowchart LR\n  A[Source] --> B[Layout] --> C[Text]\n' | boxmaid
```

Or choose a [release binary](https://github.com/rickthegreatest/boxmaid/releases/tag/v0.1.0) for Linux, macOS, or Windows. Archives include an MIT license and README; `SHA256SUMS` contains download checksums.

The Rust CLI and library are also published on [crates.io](https://crates.io/crates/boxmaid):

```sh
cargo install boxmaid --version 0.1.0 --locked
```

For reproducible agent/CI previews:

```sh
boxmaid -w 80 -color never -require-fit diagram.mmd
```

## Where can I learn more?

- [Project showcase](https://rickthegreatest.github.io/boxmaid/)
- [Quick start and APIs](https://github.com/rickthegreatest/boxmaid#readme)
- [Agent integration guide](https://github.com/rickthegreatest/boxmaid/blob/main/docs/agents.md)
- [Architecture and implementation](https://github.com/rickthegreatest/boxmaid/blob/main/docs/architecture.md)
- [Rust API documentation](https://docs.rs/boxmaid)
- [Changelog](https://github.com/rickthegreatest/boxmaid/blob/main/CHANGELOG.md)

## How can I help?

Try a diagram you already use. If something draws badly, [open an issue](https://github.com/rickthegreatest/boxmaid/issues) with the Mermaid source, terminal width, and expected result. Examples from real CLI and TUI integrations are welcome.
