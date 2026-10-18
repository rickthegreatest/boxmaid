# How should an agent work on Boxmaid?

Boxmaid renders a documented subset of Mermaid to terminal text. This repository contains a dependency-free Go core/CLI, a separate Go Bubble Tea module (`tui/`), and a Rust crate/CLI with optional Ratatui support (`rust/`).

## What should I read first?

- `README.md`: public behavior, commands, installation, and limitations.
- `docs/architecture.md`: implementation boundaries and invariants.
- `docs/syntax.md`: exact syntax coverage; unsupported statements can be skipped.
- `docs/agents.md`: integration recipes and error handling.
- `CONTRIBUTING.md`: verification commands and cross-language rules.

## Which architectural rules matter?

- Prefer composition and concrete types. Go and Rust have no class inheritance; do not introduce inheritance-like frameworks for reuse.
- Keep the Go root focused on its named public types/entry points (`boxmaid.go`), styles (`style.go`), and public contract tests. Parsers, layout, canvas, and their private tests live in `internal/renderer/`.
- Preserve public Go type identities and method sets. Replacing a named public type with an alias to an internal type is an API change; check compatibility when moving implementation code.
- Graph-family parsers produce the shared graph model. Keep layout/routing out of parsers and terminal/framework dependencies out of the core.
- Reuse the canvas, label normalization, width/grapheme handling, dates, and numeric/entity helpers. Never truncate display labels by byte offsets.
- Keep graph membership consistent through `moveNode` / `move_node`.
- Preserve deterministic declaration order. Map iteration must not affect rendered output.
- Keep Go and Rust rendering changes together. Both must match shared goldens; `LiveURL` compression bytes are the documented exception.
- Fitting is best-effort. Never silently claim a width guarantee or complete Mermaid validation.
- Preserve user changes; do not commit, tag, push, or publish unless requested.

## How do I verify a change?

Run from the repository root:

```sh
go vet ./... ./tui/...
go test -race ./... ./tui/...
cargo fmt --manifest-path rust/Cargo.toml --check
cargo clippy --manifest-path rust/Cargo.toml --all-targets --all-features -- -D warnings
cargo test --manifest-path rust/Cargo.toml --all-features
go run ./internal/artwork -check
```

`go.work` resolves the unreleased core locally. Do not put a local `replace` in `tui/go.mod`; consumers need the real versioned dependency.

For intended drawing changes, add a minimal `.mmd` fixture under `testdata/cases/`, run `go test -run Golden -update`, inspect every changed golden, and implement the matching Rust change. Run CLI parity using `scripts/check-parity.py` as described in `CONTRIBUTING.md`.

For README artwork changes, edit `docs/examples/` or `internal/artwork/`, regenerate with `go run ./internal/artwork`, and inspect the images. Do not hand-edit generated SVGs.

## What should a final summary say?

Describe concrete changes, checks actually run, and any checks blocked by the environment. Do not claim exhaustive audits, full Mermaid compliance, or release publication without evidence.
