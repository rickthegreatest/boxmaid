# Contributing

Thanks for helping! A few things keep boxmaid what it is.

## One output, two languages

The Go module and the Rust crate must draw every input to exactly the same text. So a change to the drawing has to land in both, in the same pull request. The goldens enforce this:

1. Make the change in Go and run `go test ./...`.
2. If the output changed on purpose, regenerate with `go test -run Golden -update` and review the diff in `testdata/golden/`.
3. Make the same change in `rust/`. `cd rust && cargo test` has to pass against the new goldens, byte for byte.

When you add a case, put the source in `testdata/cases/<name>.mmd`. Both test suites pick it up automatically.

Some things need care to keep the two identical:

- Go's integer division and `int(float)` truncate toward zero, so in Rust use `i64` arithmetic and `as i64`.
- Rust's regex `\w \s \d \b` are Unicode-aware but Go's are ASCII. Compile patterns with `re::lazy_re!`, which rewrites them.
- Sorts must be stable: Go's `sort.SliceStable` corresponds to Rust's `sort_by`.
- Never let map iteration order reach the output.
- Character widths, HTML entities, date maths and number formatting come from shared tables and our own code, not the platform. Use them.

`LiveURL` is the one exception: both produce valid mermaid.live links, but the compressed bytes differ.

## Style

- Go: `gofmt`, `go vet`. The core module stays dependency-free.
- Rust: `cargo fmt` (see `rustfmt.toml`) and `cargo clippy --all-features -- -D warnings`. The only runtime dependency is `regex` (plus `ratatui` behind its feature).
- Match the surrounding code: short doc comments that say why, not what.

## Reporting a diagram that draws badly

Open an issue with the Mermaid source, the width you drew it at, and what you expected. A case for `testdata/cases/` is the most useful fix you can send.
