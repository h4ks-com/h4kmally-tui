# h4kmally-tui

Terminal agar.io client built with Go and Bubble Tea v2. Connects to [h4kmally-server](https://github.com/h4ks-com/h4kmally-server).

## Features

- 🎮 Full agar.io gameplay in your terminal
- 🖱️ Mouse-based movement
- ⚡ Real-time WebSocket protocol (SIG 0.0.1)
- 🎨 Unicode cell rendering with size indicators
- 🔒 WSS (secure WebSocket) support
- 🐳 Docker Compose for easy testing

## Installation

### From Source

```bash
git clone https://github.com/h4ks-com/h4kmally-tui.git
cd h4kmally-tui
go build -o h4kmally-tui ./cmd/tui-agar
```

### Docker

```bash
docker build -t h4kmally-tui .
docker run -it --rm h4kmally-tui -server wss://your-server/ws/ -name Player
```

## Quick Start

```bash
# Start a local test server
docker compose up -d server

# Run the client
./h4kmally-tui -name YourName

# Connect to a custom server (note the trailing slash!)
./h4kmally-tui -server wss://api.sigmally.h4ks.com/ws/ -name YourName
```

## Controls

| Key | Action |
|-----|--------|
| **Mouse** | Move toward cursor |
| **Space** | Split |
| **W** | Eject mass |
| **R** | Respawn (when dead) |
| **Q** | Quit |

## Testing

```bash
# Test connection without TUI
go run ./cmd/test-client -server wss://api.sigmally.h4ks.com/ws/ -name TestBot
```

## Cell Rendering

Cells are rendered using Unicode characters based on their screen size:

| Size | Character | Description |
|------|-----------|-------------|
| Tiny | `·` | Food/small cells |
| Small | `•` | Small players |
| Medium | `○` | Medium players |
| Large | `●` | Large players |
| Huge | `⬤` | Very large players |
| **Yours** | `◉` | Your cells (highlighted) |

## Architecture

```
┌─────────────────┐
│   TUI Layer     │  Bubble Tea v2
│  (internal/tui) │  Mouse handling, rendering
└────────┬────────┘
         │
┌────────▼────────┐
│  Game Engine    │  Pure Go, fully testable
│  (internal/game)│  World state, camera
└────────┬────────┘
         │
┌────────▼────────┐
│   Protocol      │  SIG 0.0.1 binary
│(internal/protocol) Opcode shuffling
└────────┬────────┘
         │
┌────────▼────────┐
│  WebSocket      │  gorilla/websocket
│  (internal/client)│
└─────────────────┘
```

## Protocol

Implements the SIG 0.0.1 binary WebSocket protocol:

- Little-endian byte order
- Null-terminated UTF-8 strings
- JSON payloads for complex messages
- Opcode shuffling (256-byte table sent at handshake)

See [PROTOCOL.md](https://github.com/h4ks-com/h4kmally-server/blob/main/PROTOCOL.md) for full spec.

## Development

```bash
# Run locally
go run ./cmd/tui-agar

# Build
make build

# Start dev server + client
make dev

# Run tests
go test ./...
```

## Requirements

- Go 1.22+
- Terminal with Unicode support
- (Optional) Docker for test server

## Related Projects

- [h4kmally-server](https://github.com/h4ks-com/h4kmally-server) - The game server
- [h4kmally-client](https://github.com/h4ks-com/h4kmally-client) - Web client

## License

[MIT](LICENSE)

## Contributing

PRs welcome! Please open an issue first to discuss major changes.
