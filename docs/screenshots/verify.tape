# vhs tape: GoLogX audit core — records what an AI agent did into a signed,
# hash-chained log, verifies it intact, then forges a single byte and shows
# `verify` catch it at the exact entry.
# Build with: vhs docs/screenshots/verify.tape (run from repo root)

Output docs/screenshots/_verify_raw.gif

Set Shell           "bash"
Set FontSize        15
Set Width           1300
Set Height          360
Set Padding         28
# No window chrome / margin — gen.sh composites the atlas-ragnarok
# shader vignette on top in post; the recording needs to be edge-to-edge
# for the overlay to sit flush.
# atlas-ragnarok — https://github.com/AyoubTadlaoui/atlas-ragnarok
# Pure black bg, white fg, thunder-blue accents, peppermint greens, crimson reds.
Set Theme           { "name": "atlas-ragnarok", "background": "#000000", "foreground": "#FFFFFF", "cursor": "#3B82F6", "selection": "#1E3A5F", "black": "#000000", "red": "#FF8080", "green": "#99FFE4", "yellow": "#2563EB", "blue": "#A0A0A0", "purple": "#3B82F6", "cyan": "#99FFE4", "white": "#FFFFFF", "brightBlack": "#505050", "brightRed": "#FF9999", "brightGreen": "#B3FFE4", "brightYellow": "#60A5FA", "brightBlue": "#B0B0B0", "brightPurple": "#93C5FD", "brightCyan": "#99FFE4", "brightWhite": "#FFFFFF" }
Set TypingSpeed     20ms
Set PlaybackSpeed   1.0
# Slower per-frame delay (10 cs ≈ 100ms) helps Safari render reliably.
Set Framerate       10

# Setup (hidden): PATH + a clean prompt, and warm the build cache so the
# on-screen `go run` is instant.
Hide
Type 'export PATH="/opt/homebrew/bin:/usr/local/go/bin:$PATH"; PS1="$ "'
Enter
Sleep 200ms
Type 'go build ./examples/audit'
Enter
Sleep 1200ms
Type 'clear'
Enter
Sleep 200ms
Show

# Visible: the whole tamper-evident story in one command. The example records
# three agent actions into a signed, hash-chained log, verifies it, then
# rewrites "loadsh" -> "lodash" to hide a blocked typosquat and re-verifies.
Type 'go run ./examples/audit'
Sleep 500ms
Enter

# Let the output render and breathe.
Sleep 5000ms
