# How can I embed Mermaid in Bubble Tea?

`github.com/rickthegreatest/boxmaid/tui` is a pannable Mermaid diagram component for **Bubble Tea v1** (`github.com/charmbracelet/bubbletea`). It wraps the Boxmaid renderer with Lip Gloss styling, width-aware layout, keyboard navigation, and mouse-wheel panning.

![A real Bubble Tea runbook switches between checkout, request, and service diagrams, and toggles the Mermaid source.](../docs/assets/runbook.gif)

## How do I add the component?

```sh
go get github.com/rickthegreatest/boxmaid/tui@v0.1.1
```

```go
diagram := tui.New(src)
diagram.SetSize(width, height)
diagram.SetStyles(tui.DefaultStyles())

// In the containing application's Update:
diagram, cmd = diagram.Update(msg)

// In View:
view := diagram.View()
```

Your application owns quitting and the diagram's allocated size. Call `SetSize` on resize and `SetSource` when selecting another diagram. Use `SetOptions` to redraw with different ASCII/rendering options. `Err` reports rendering errors, `Picture` exposes the cell grid, and `Offset` returns the scroll position.

## Can I run a complete example?

From the repository root, with Go 1.24+:

```sh
go run ./tui/examples/runbook
```

Press **1–3** to select a runbook diagram, **s** to toggle source/drawing, arrows or **hjkl** to pan, and **q** to quit. The sidebar collapses in narrow terminals. The embedded diagrams are synthetic examples; no service or network access is required.

See the [complete runbook source](examples/runbook/main.go), [embedding walkthrough](../docs/demo.md), or [single-file viewer](examples/viewer/main.go).

## Which Mermaid syntax is supported?

See [syntax coverage](../docs/syntax.md). Boxmaid renders a selective subset, and width fitting is best-effort. The component provides scrolling when the drawing still exceeds its viewport.
