# Run from the repository root with VHS, ttyd, ffmpeg, and a Chromium browser.
Output docs/assets/runbook.gif
Set Shell "bash"
Set Width 1120
Set Height 560
Set FontSize 18
Set Padding 24
Set Framerate 10
Set Theme "Catppuccin Mocha"
Hide
Type "go run ./tui/examples/runbook"
Enter
Sleep 4s
Show
Sleep 2s
Type "2"
Sleep 2s
Type "s"
Sleep 2s
Screenshot docs/assets/runbook-source.png
Sleep 300ms
Type "s"
Type "3"
Sleep 2s
Screenshot docs/assets/runbook.png
Sleep 300ms
Type "1"
Sleep 2s
Hide
Type "q"
