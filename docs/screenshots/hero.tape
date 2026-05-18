# vhs tape: render the static hero shot.
# vhs is used (instead of freeze) because vhs's per-tape theme JSON controls
# the actual ANSI palette, while freeze's chroma themes only touch syntax
# highlighting — `freeze --execute` ignores ANSI palette overrides.
#
# Build with: vhs docs/screenshots/hero.tape

Output docs/screenshots/_hero_raw.png

Set Shell           "bash"
Set FontSize        14
Set Width           1100
Set Height          400
Set Padding         28
# No window chrome / margin — gen.sh composites the atlas-ragnarok
# shader vignette (thunder-blue top glow + crimson bottom glow + radial
# vignette) on top in post, so the recording needs to be edge-to-edge
# for the overlay to sit flush over the whole canvas.
# atlas-ragnarok — https://github.com/AyoubTadlaoui/atlas-ragnarok
Set Theme           { "name": "atlas-ragnarok", "background": "#000000", "foreground": "#FFFFFF", "cursor": "#3B82F6", "selection": "#1E3A5F", "black": "#000000", "red": "#FF8080", "green": "#99FFE4", "yellow": "#2563EB", "blue": "#A0A0A0", "purple": "#3B82F6", "cyan": "#99FFE4", "white": "#FFFFFF", "brightBlack": "#505050", "brightRed": "#FF9999", "brightGreen": "#B3FFE4", "brightYellow": "#60A5FA", "brightBlue": "#B0B0B0", "brightPurple": "#93C5FD", "brightCyan": "#99FFE4", "brightWhite": "#FFFFFF" }

# Run logx against the sample stream, wait for it to settle, screenshot.
Hide
Type 'export PATH="/opt/homebrew/bin:$PATH" && clear'
Enter
Sleep 100ms
Show

Type 'logx -level=debug docs/screenshots/sample.json'
Sleep 300ms
Enter
Sleep 800ms

Screenshot docs/screenshots/_hero_raw.png
Sleep 200ms
