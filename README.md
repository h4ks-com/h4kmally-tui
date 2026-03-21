# h4kmally-tui

Terminal agar.io client written in Go. Connects to the [h4kmally server](https://github.com/h4ks-com/h4kmally-server).

## Quick start

```bash
git clone https://github.com/h4ks-com/h4kmally-tui.git
cd h4kmally-tui
go run ./cmd/tui-agar -name yourname -server wss://api.sigmally.h4ks.com/ws/
```

Requires Go 1.24+ and a terminal with Unicode and 24-bit color support.

## Controls

| Key | Action |
|-----|--------|
| Mouse | Move toward cursor |
| Space | Split |
| W | Eject mass |
| R | Respawn (when dead) |
| Q | Quit |

## Cell rendering

| Size | Character | Description |
|------|-----------|-------------|
| Tiny | `·` | Food / small cells |
| Small | `•` | Small players |
| Medium | `○` | Medium players |
| Large | `●` | Large players |
| Huge | `⬤` | Very large players |
| Yours | `◉` | Your cells (highlighted) |

## Related

- [h4kmally-server](https://github.com/h4ks-com/h4kmally-server) — game server
- [h4kmally-client](https://github.com/h4ks-com/h4kmally-client) — web client
