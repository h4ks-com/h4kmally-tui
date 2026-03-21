# h4kmally-tui

Terminal agar.io client for the h4kmally server. Written in Go using Bubble Tea v2.

## What it does

Connects to `wss://api.sigmally.h4ks.com/ws/`, speaks SIG 0.0.2 binary WebSocket protocol, renders the game in the terminal with 24-bit ANSI colors, leaderboard panel, and minimap.

## Run

```sh
go run ./cmd/tui-agar -name yourname -server wss://api.sigmally.h4ks.com/ws/
```

Keys: Mouse=aim, Space=split, W=eject, R=respawn, Q=quit.

## Code layout

```
internal/protocol/protocol.go   — SIG 0.0.2 encode/decode (all wire format here)
internal/game/game.go           — world state, cells, camera, zoom, minimap math
internal/client/client.go       — WebSocket connection, gorilla/websocket
internal/tui/tui.go             — Bubble Tea model, rendering (colors, leaderboard, minimap)
cmd/tui-agar/main.go            — entrypoint, flags
cmd/test-client/main.go         — headless client, prints all messages (good for protocol debugging)
cmd/debug-pty/main.go           — runs tui-agar in a PTY and dumps raw output
```

## Build & test

```sh
go build ./...                  # must pass clean

# Run the TUI
go run ./cmd/tui-agar -name test -server wss://api.sigmally.h4ks.com/ws/

# Test protocol messages against the real server (headless, prints decoded messages)
go run ./cmd/test-client -server wss://api.sigmally.h4ks.com/ws/ -name test

# Render test via tmux (captures terminal output without a real terminal)
go build -o tui-agar ./cmd/tui-agar
tmux new-session -d -s tui_test -x 120 -y 35
tmux send-keys -t tui_test './tui-agar -name test -server wss://api.sigmally.h4ks.com/ws/' Enter
sleep 4 && tmux capture-pane -t tui_test -p
tmux kill-session -t tui_test

# Run in PTY and dump raw ANSI output
go run ./cmd/debug-pty -name test -duration 5
```

## Unit tests

```sh
go test ./...
```

Tests live next to the code they test (`_test.go` files). Priority areas:
- `internal/protocol/` — decode/encode round-trips (most critical, pure functions)
- `internal/game/` — zoom formula, coordinate transforms, visible cell filtering

To add a protocol test: write a byte buffer matching the spec, call `DecodeMessage`, assert the result. The server source at `/tmp/h4kmally-server/internal/protocol/` has the canonical encoder to cross-reference.

## Protocol

**Spec**: `https://github.com/h4ks-com/h4kmally-server/blob/main/PROTOCOL.md`
**Server source**: `/tmp/h4kmally-server/internal/protocol/` (cloned locally)

Key facts:
- Handshake: client sends `"SIG 0.0.2\0"`, server responds with same string + 256-byte shuffle table
- All opcodes are XOR-shuffled per connection via that table
- Mouse sends `i32` world coords (not relative, not float)
- World update: eat events → cells (sentinel 0x00000000) → removals
- Cell coords: `i16` x/y, `u16` size (radius). Flags byte controls optional fields (color=0x02, skin=0x04, name=0x08, effect=0x10)
- Camera zoom is always 0 from server — client computes zoom from player cell size
- Leaderboard (opcode 49): `u32 count`, then per entry: `u32 isMe, string name, u32 rank, u32 isSubscriber`

## Debugging

- `/tmp/tui-internal.log` — tui.go debug log (message types, state transitions)
- `/tmp/agar-debug.log` — main entrypoint log
- `/tmp/pty-debug.log` — debug-pty log

## Notes

- The server is `api.sigmally.h4ks.com` (h4ks custom server), NOT the standard sigmally.com servers
- `ViewZoom()` in game.go computes zoom: target is ~6 char radius for player cell regardless of mass
- Minimap grid: world is ±14142, divided 7×7 (columns A-G, rows 1-7)
- Colors are 24-bit ANSI (`\x1b[38;2;R;G;Bm`) from the RGB sent per cell in world updates
