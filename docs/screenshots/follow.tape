# vhs tape: shows `logx -f` tailing a JSON file as new records appear.
# Build with: vhs docs/screenshots/follow.tape
#
# Trick: vhs only has one terminal, so we spawn the appender as a BACKGROUND
# shell BEFORE starting `logx -f` in the foreground. The appender writes one
# JSON record every 0.7s; logx picks them up live.

Output docs/screenshots/follow.gif

Set Shell           "bash"
Set FontSize        15
Set Width           1300
Set Height          560
Set Padding         24
# atlas-ragnarok — https://github.com/AyoubTadlaoui/atlas-ragnarok
# Pure black bg, white fg, thunder-blue accents, peppermint greens, crimson reds.
Set Theme           { "name": "atlas-ragnarok", "background": "#000000", "foreground": "#FFFFFF", "cursor": "#3B82F6", "selection": "#1E3A5F", "black": "#000000", "red": "#FF8080", "green": "#99FFE4", "yellow": "#2563EB", "blue": "#A0A0A0", "purple": "#3B82F6", "cyan": "#99FFE4", "white": "#FFFFFF", "brightBlack": "#505050", "brightRed": "#FF9999", "brightGreen": "#B3FFE4", "brightYellow": "#60A5FA", "brightBlue": "#B0B0B0", "brightPurple": "#93C5FD", "brightCyan": "#99FFE4", "brightWhite": "#FFFFFF" }
Set TypingSpeed     20ms
Set PlaybackSpeed   1.0
# Slower per-frame delay (10 cs ≈ 100ms) helps Safari render reliably.
Set Framerate       10

# Setup (hidden): clean file, ensure PATH, spawn the background appender.
Hide
Type 'export PATH="/opt/homebrew/bin:$PATH" && rm -f /tmp/demo.json && touch /tmp/demo.json'
Enter
Sleep 200ms
Type `(sleep 1.0; echo '{"time":"2026-05-18T09:14:02.001Z","level":"INFO","msg":"server up","service":"api","port":8080,"env":"prod"}' >> /tmp/demo.json; sleep 0.7; echo '{"time":"2026-05-18T09:14:02.183Z","level":"DEBUG","msg":"warming caches","items":1024,"ttl_s":300}' >> /tmp/demo.json; sleep 0.7; echo '{"time":"2026-05-18T09:14:02.402Z","level":"INFO","msg":"request","method":"GET","path":"/v1/users/42","status":200,"dur_ms":12}' >> /tmp/demo.json; sleep 0.7; echo '{"time":"2026-05-18T09:14:03.115Z","level":"WARN","msg":"slow query","query":"orders.list","ms":230,"rows":4812}' >> /tmp/demo.json; sleep 0.7; echo '{"time":"2026-05-18T09:14:03.881Z","level":"ERROR","msg":"upstream failed","service":"payments","err":"timeout after 5s","retries":3,"trace_id":"4f9c1e2a"}' >> /tmp/demo.json) &`
Enter
Sleep 200ms
Type 'clear'
Enter
Sleep 200ms
Show

# Visible: start tailing. The background job will start writing in ~1s.
Type 'logx -f /tmp/demo.json'
Sleep 400ms
Enter

# Let the appender finish its sequence (5 lines × 0.7s + 1s initial = ~4.5s),
# plus a generous buffer for the eye.
Sleep 6000ms

# Stop following.
Ctrl+C
Sleep 800ms
