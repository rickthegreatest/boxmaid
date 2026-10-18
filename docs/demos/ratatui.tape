Output docs/assets/ratatui.gif
Set Shell "bash"
Set Width 1040
Set Height 360
Set FontSize 18
Set Padding 24
Set Framerate 10
Set Theme "Catppuccin Mocha"
Hide
Type "cargo run --quiet --locked --manifest-path rust/Cargo.toml --example ratatui_viewer --features ratatui -- tui/examples/runbook/diagrams/services.mmd"
Enter
Sleep 5s
Show
Sleep 3s
Screenshot docs/assets/ratatui.png
Sleep 1s
Hide
Type "q"
