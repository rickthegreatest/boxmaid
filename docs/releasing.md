# How is Boxmaid released?

The repository publishes three packages: a Go core/CLI, a separate Go TUI module, and a Rust crate/CLI. GitHub release archives contain the Go CLI; both implementations expose the same command contract. This guide describes publication; repository preparation alone does not publish anything.

## Which versions and paths must agree?

For the current release:

| Item | Value |
|---|---|
| Repository | `https://github.com/rickthegreatest/boxmaid` |
| Go core tag | `v0.1.1` |
| Go TUI tag | `tui/v0.1.1` |
| Rust package version | `0.1.1` |
| Go CLI default version | `0.1.1` |
| TUI's core dependency | `github.com/rickthegreatest/boxmaid v0.1.1` |

When changing a version, update `rust/Cargo.toml`, `rust/Cargo.lock`, `cmd/boxmaid/main.go`, `tui/go.mod`, and the version-specific replacement in `go.work`. The latter is workspace-only; it must not be copied into a published module's `go.mod`.

## What should be checked before tagging?

1. Run the full checks in [CONTRIBUTING.md](../CONTRIBUTING.md), including CLI parity and artwork regeneration checks.
2. Run `cargo package --locked --manifest-path rust/Cargo.toml`. This verifies the packaged crate by compiling it outside the source tree. In a deliberately uncommitted local review, add `--allow-dirty`; a real release should be clean.
3. Check the minimum Go/Rust versions and all platform jobs in CI. Ensure `main` is the branch used by documentation links and CI.
4. Review the dated `CHANGELOG.md` entry, examples, installation paths, and packaged LICENSE/README. Remove the README's unreleased-checkout wording when the release is ready to publish.
5. Replace `unreleased` in the changelog heading with the actual release date and run `python3 scripts/check-release.py v0.1.1`.
6. Confirm the `boxmaid` crate name is available to your crates.io account. A GitHub repository name does not reserve a crates.io name.

The version check deliberately refuses an unreleased changelog entry. Keep it unreleased while still preparing the release. To verify metadata before publication, run `python3 scripts/check-release.py v0.1.1 --allow-unreleased`; the tag-triggered workflow does not use this override.

## In what order should I publish?

1. Push the reviewed repository to `rickthegreatest/boxmaid` and let CI pass.
2. Create and push the core tag `v0.1.1` on the reviewed commit. The tag triggers `.github/workflows/release.yml`.
3. The release workflow verifies code, packaged Rust compilation, artwork, and CLI parity. It builds Go binaries for Linux/macOS/Windows on amd64/arm64, bundles README/LICENSE, creates SHA-256 checksums, and publishes the GitHub release.
4. Verify `GOWORK=off go install github.com/rickthegreatest/boxmaid/cmd/boxmaid@v0.1.1` from outside the checkout.
5. Create/push `tui/v0.1.1`. Verify `go get github.com/rickthegreatest/boxmaid/tui@v0.1.1` from a temporary external module. The core tag must be accessible first.
6. Publish Rust deliberately with `cargo publish --locked --manifest-path rust/Cargo.toml`, then verify `cargo install boxmaid --version 0.1.1 --locked` from the registry.

Go modules become available from tags; no separate package upload is needed. Registry publication is not automatic in the GitHub workflow, and no registry token is required for repository CI.

## How do I verify a downloaded binary?

Download its archive and `SHA256SUMS` from the same release. On Linux, use `sha256sum --check --ignore-missing SHA256SUMS`. On macOS, use `shasum -a 256` and compare the value for your archive. Windows users can use `Get-FileHash -Algorithm SHA256`.

Extract the archive and run `boxmaid -version`, `boxmaid -help`, and a small diagram. Windows archives contain `boxmaid.exe`; macOS/Linux archives contain `boxmaid`. Both language installations use this same executable name, so install only the implementation you intend to invoke.

## What does package verification cover?

The Rust package includes its own README, MIT license, source, and examples. The integration tests in `rust/tests/` depend on repository-level fixtures and are intentionally excluded from the crate archive; run them in the checkout before packaging. `cargo package` compiles the packaged crate but does not substitute for those tests.

GitHub Actions cannot be executed locally as hosted jobs. Local cross-compilation validates buildability; only the native OS jobs establish runtime test results for those platforms.

## Who can update the public repository?

The active `Owner-only branches` and `Owner-only tags` GitHub rulesets restrict creation, updates, and deletion across all branches and tags to the repository admin. `rickthegreatest` is the sole admin and write-access account. Public readers can fork and propose pull requests, but cannot push or merge. Workflows default to read-only permissions and cannot approve pull-request reviews.

The owner can publish release tags normally through the admin bypass; release automation uploads artifacts without changing protected refs. Repository access and rulesets are managed in GitHub settings, independently of the source files.
