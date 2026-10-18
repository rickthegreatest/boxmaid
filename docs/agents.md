# How should an agent integrate Boxmaid?

Boxmaid converts a selective subset of Mermaid to terminal text. It has two equivalent CLIs, a Go library, a Rust crate, and optional TUI adapters. Use it for text previews and programmatic rendering; it is not a full Mermaid validator.

## What is the fastest reliable command?

```sh
boxmaid -w 80 -color never -require-fit diagram.mmd
```

This fixes the column budget, removes ANSI styling, and returns exit `3` if the drawing cannot fit. For maximum character compatibility, add `-ascii`. Flags must precede filenames. Pipe source to stdin, pass `-`, or pass file paths; use `--` before paths beginning with `-`.

During development, replace `boxmaid` with `go run ./cmd/boxmaid` or `cargo run --manifest-path rust/Cargo.toml --`. Published binaries avoid build-system diagnostics in stderr.

## How do I interpret the result?

| Result | Required handling |
|---|---|
| Exit `0` | Consume stdout. Without `-require-fit`, also inspect stderr for overflow. |
| Exit `1` | A render/output failed. Keep the original source; show the diagnostic or a browser fallback. |
| Exit `2` | Correct arguments, input path, or input encoding. |
| Exit `3` | Increase `-w`, simplify the diagram, or use a scrollable viewer. The oversized diagram was not printed. |

Diagnostics go to stderr, not stdout. Multiple inputs can produce partial stdout before a later failure; the command is not transactional. `-md` preserves unsupported blocks and does not turn those blocks into a nonzero exit. A consumer closing a pipe early is handled without a panic.

An exit `0` is **not** proof of faithful source interpretation: unknown statements inside supported diagrams are generally skipped. `boxmaid -list` lists accepted headers, not supported statements.

## Which interface should I choose?

- **External tools / coding agents:** CLI with explicit width and `-color never`.
- **Go applications:** `boxmaid.Render`; inspect `Picture.W`, `H`, and `Cells`.
- **Rust applications:** `boxmaid::render`; inspect `Picture.w`, `h`, and `cells`.
- **Bubble Tea:** `tui.New`, `SetSize`, `Update`, `View`. Use `SetStyles` and `SetOptions` to refresh cached output.
- **Ratatui:** render `&Picture`, or use `Diagram::new(&picture).theme(...).scroll(x, y)`. Repeat layout when the available width changes.
- **Markdown pipelines:** `ReplaceMermaid` / `replace_mermaid`, or CLI `-md`. Output is plain fenced text.

`Picture.String()` / `Display` excludes the title. The CLI prints title metadata above the drawing. The library does no terminal detection or automatic ANSI emission.

## How do I handle unsupported input in Go?

```go
pic, err := boxmaid.Render(src, boxmaid.Options{Width: 80})
if err != nil {
    var unsupported *boxmaid.UnsupportedError
    if errors.As(err, &unsupported) {
        return fmt.Errorf("unsupported %s; browser view: %s", unsupported.Kind, boxmaid.LiveURL(src))
    }
    return err
}
if pic.W > 80 {
    return fmt.Errorf("diagram needs %d columns", pic.W)
}
```

Go also exposes `ErrEmpty` and `ErrInvalidUTF8`. Other parse failures have descriptive error messages; do not branch on their wording. Rust's `&str` enforces valid UTF-8 before library rendering; both CLIs reject malformed byte input.

## How do I handle unsupported input in Rust?

```rust
match boxmaid::render(src, &boxmaid::Options { width: 80, ascii: false }) {
    Ok(pic) if pic.w <= 80 => println!("{pic}"),
    Ok(pic) => eprintln!("diagram needs {} columns", pic.w),
    Err(boxmaid::Error::Unsupported(kind)) => {
        eprintln!("unsupported {kind}; browser view: {}", boxmaid::live_url(src));
    }
    Err(err) => eprintln!("{err}"),
}
```

Rust exposes `Error::Empty`, `Unsupported`, and `Parse`. `supported` / Go `Supported` only recognize a header; they do not parse or validate the entire source.

## What affects reproducibility?

Pin the package version and specify width, ASCII mode, and colour policy. Plain text output is independent of terminal detection when these are explicit. Shared goldens compare Go and Rust drawings; CI also compares full CLI stdout, stderr, and status for rendering fixtures.

Terminal glyph widths vary by terminal/font. Boxmaid uses its own shared tables for common CJK/emoji/grapheme cases, not a platform font renderer. Do not use byte lengths to crop a cell grid. Use `StringWidth` / `string_width` for the library's measurement model.

For source compatibility decisions, consult [syntax coverage](syntax.md). Pie/radar/Sankey representations are terminal adaptations, not the same visual shape as a browser chart.

## Does rendering execute Mermaid actions or contact a service?

No. Rendering is local. Mermaid actions such as `click` are not executed. `LiveURL` / `live_url` / `-live` embeds the source in a URL without making an HTTP request or launching a browser. Retain the source yourself when you need fallbacks.

## Where is the implementation contract?

[Architecture](architecture.md) describes the data flow. [AGENTS.md](../AGENTS.md) gives repository rules, and [CONTRIBUTING.md](../CONTRIBUTING.md) gives verification commands. Public import paths are `github.com/rickthegreatest/boxmaid`, `github.com/rickthegreatest/boxmaid/tui`, and Rust crate `boxmaid`.
