# How can I contribute?

Small, reproducible diagram fixes are especially welcome. Include the source, requested width, actual drawing, and expected behavior. For implementation work, read [architecture](docs/architecture.md) and [syntax coverage](docs/syntax.md) first.

## What tools do I need?

Go 1.24+ for the full workspace (the core alone supports 1.22), Rust 1.88+, and Python 3.9+ for repository/release scripts. The Go core has no third-party dependencies. Rust uses `regex`, with optional Ratatui. Work from the repository root unless a command says otherwise.

`go.work` resolves the core and TUI module locally, including the first unreleased core version. `tui/go.mod` has a real tagged dependency and no local replacement, so downstream consumers can use it normally after publication.

## Where should I put a change?

- **Public Go API or styling:** `boxmaid.go` or `style.go`. Keep existing named types and method sets compatible.
- **Go parsing, layout, and drawing:** `internal/renderer/`, with implementation tests beside the code.
- **Public golden/fuzz contracts:** `boxmaid_test.go`; the shared corpus remains under `testdata/`.
- **CLI, TUI, Rust, documentation:** `cmd/boxmaid/`, `tui/`, `rust/`, and `docs/` respectively.

See [the architecture map](docs/architecture.md) for the full package boundaries. Splitting a parser into an additional package should serve a real dependency boundary rather than just shorten a file listing.

## Which checks should I run?

```sh
gofmt -w .
go vet ./... ./tui/...
go test -race ./... ./tui/...
cargo fmt --manifest-path rust/Cargo.toml --check
cargo clippy --manifest-path rust/Cargo.toml --locked --all-targets --all-features -- -D warnings
cargo test --manifest-path rust/Cargo.toml --locked --all-features
go run ./internal/artwork -check
```

For CLI parity (example paths for macOS/Linux):

```sh
go build -o /tmp/boxmaid-go ./cmd/boxmaid
cargo build --locked --manifest-path rust/Cargo.toml
python3 scripts/check-parity.py --go /tmp/boxmaid-go --rust rust/target/debug/boxmaid
```

The script compares fixture rendering at unlimited, 30, 60, and 120 columns in Unicode/ASCII, plus colour themes, status codes, Markdown boundaries, and decoded browser links. On Windows, choose a temporary `.exe` path and pass `rust/target/debug/boxmaid.exe`.

CI also checks the minimum language versions and runs Go/Rust tests on Linux, macOS, and Windows. A local run only establishes compatibility with the local platform/toolchains.

## How do I change a drawing?

1. Add a minimal `.mmd` case under `testdata/cases/` and an assertion for the behavior when the fixture alone would not expose the bug.
2. Implement the fix in Go and Rust in the same change.
3. Run `go test -run Golden -update` to regenerate shared fixtures.
4. Inspect **every changed drawing** in `testdata/golden/`; do not accept snapshots blindly.
5. Run both test suites and CLI parity. If artwork changed, regenerate it and inspect it too.

Goldens include Unicode at width 100 and ASCII at width 80. Both language suites discover new cases automatically. Drawing changes can legitimately alter many snapshots; explain the actual layout improvement.

## What keeps Go and Rust aligned?

- Integer division and float-to-integer conversion truncate toward zero. Use signed intermediate layout arithmetic where needed in Rust.
- Rust's regex shorthand classes are Unicode-aware; Go's are ASCII. Use `re::lazy_re!` for portable patterns.
- Stable sorts and source order matter. Never let unordered map iteration determine output.
- Reuse shared-width behavior, portable dates, entities, and number formatting. Fix incorrect behavior in both implementations rather than copying a language-specific bug.
- Browser-link compressed bytes may differ, but must decode to equivalent source state.

## What code style should I follow?

Prefer straightforward code, composition, explicit ownership, and small helpers with real callers. Keep graph semantics in the graph model, geometry in layout/drawing, and framework/terminal behavior in adapters. Avoid speculative abstraction layers and unrelated cleanup.

Do not add dependencies to the Go core for functionality already available in the standard library. Explain new runtime dependencies. Keep comments about contracts, invariants, and non-obvious decisions rather than narrating statements.

## How do I update the images?

Edit Mermaid source in `docs/examples/` or presentation code in `internal/artwork/`, then run:

```sh
go run ./internal/artwork
go run ./internal/artwork -check
```

Generated SVGs go in `docs/assets/`. They use real renderer geometry and semantic cell styles; they are not manually drawn diagrams. Inspect at README display size, check readable contrast, and keep descriptive alt text. Do not hand-edit generated SVGs.

## How are releases made?

Follow [the release guide](docs/releasing.md). Core Go and `/tui` need separate tags, and the core must be available before the TUI dependency can resolve outside this workspace.
