# Keep service diagrams beside your terminal runbook

Mermaid is useful for explaining a request path or a service boundary. Those diagrams can also sit inside the terminal tool you use to inspect that system.

This small **Boxmaid + Bubble Tea runbook** switches between a checkout flowchart, a request sequence, and service dependencies. Press `s` to see the original Mermaid source, then switch back to the drawing.

![An actual terminal recording of the Boxmaid runbook, including diagram selection and source view.](https://raw.githubusercontent.com/rickthegreatest/boxmaid/main/docs/assets/runbook.gif)

## How can I try it?

With Go 1.24+:

```sh
git clone https://github.com/rickthegreatest/boxmaid.git
cd boxmaid
go run ./tui/examples/runbook
```

Choose diagrams with `1`–`3`, toggle source with `s`, pan with arrows or `hjkl`, and quit with `q`. The example uses synthetic embedded diagrams and needs no credentials or running services.

## Can I embed it in my own tool?

Boxmaid provides a Go renderer, a Bubble Tea v1 component, a Rust crate with a Ratatui 0.29 widget, and a standalone CLI. Drawings are Unicode or ASCII text; rendering needs no browser or JavaScript runtime.

The renderer covers 22 Mermaid diagram families selectively. Unknown statements may be skipped, and fitting is best-effort, so check the [syntax reference](https://github.com/rickthegreatest/boxmaid/blob/main/docs/syntax.md) for the constructs you use.

[Walkthrough and Rust capture](https://github.com/rickthegreatest/boxmaid/blob/main/docs/demo.md) · [Example source](https://github.com/rickthegreatest/boxmaid/tree/main/tui/examples/runbook) · [Website](https://rickthegreatest.github.io/boxmaid/) · [CLI releases](https://github.com/rickthegreatest/boxmaid/releases/latest)

If you build terminal documentation, agent tools, or operational runbooks, I'd welcome a real diagram to test. Share a small Mermaid example and the column budget you need.
