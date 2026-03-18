# Architecture

## Overview

h4kmally-tui is a terminal-based agar.io client that connects to h4kmally-server via WebSocket using the SIG 0.0.1 binary protocol.

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Terminal                           │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                    TUI Layer (Bubble Tea v2)             │   │
│   │                                                          │   │
│   │  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐   │   │
│   │  │  Model   │  │  Update  │  │        View          │   │   │
│   │  │  (state) │→ │ (events) │→ │ (render to terminal) │   │   │
│   │  └──────────┘  └──────────┘  └──────────────────────┘   │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              ↑↓                                  │
└──────────────────────────────│──────────────────────────────────┘
                               │
                        ┌──────▼──────┐
                        │ Game Engine │
                        │   (Pure)    │
                        │             │
                        │ - World     │
                        │ - Cells     │
                        │ - Camera    │
                        └──────┬──────┘
                               │
                        ┌──────▼──────┐
                        │  Protocol   │
                        │  (SIG 0.0.1)│
                        │             │
                        │ - Encode    │
                        │ - Decode    │
                        │ - Shuffle   │
                        └──────┬──────┘
                               │
                        ┌──────▼──────┐
                        │  WebSocket  │
                        │   Client    │
                        └──────┬──────┘
                               │
                        ╔══════╧══════╗
                        ║   Network   ║
                        ╚══════╤══════╝
                               │
                    ┌──────────▼──────────┐
                    │   h4kmally-server   │
                    │  (or any SIG 0.0.1) │
                    └─────────────────────┘
```

## Package Structure

```
cmd/
├── tui-agar/           # Main entry point
│   └── main.go         # CLI flags, program setup
└── test-client/        # Debug tool (no TUI)
    └── main.go         # Tests protocol without terminal

internal/
├── client/             # WebSocket connection management
│   └── client.go       # Dial, read loop, message routing
│
├── protocol/           # SIG 0.0.1 binary protocol
│   └── protocol.go     # Encoding/decoding, opcode shuffling
│
├── game/               # Game state (no UI dependencies)
│   └── game.go         # World, cells, camera, coordinate transforms
│
└── tui/                # Terminal UI (Bubble Tea)
    └── tui.go          # Model, Update, View, input handling
```

## Data Flow

### 1. Connection Flow

```
Client                          Server
  │                               │
  │──── Handshake: "SIG 0.0.1\0" ────→│
  │                               │
  │←─── Handshake Response ───────│
  │     (version + 256-byte       │
  │      shuffle table)           │
  │                               │
  │──── Captcha Token ─────────────→│
  │                               │
  │──── Spawn Request ─────────────→│
  │     {name, skin, ...}         │
  │                               │
  │←─── Spawn Result ─────────────│
  │     (accepted: true/false)    │
  │                               │
  │←─── Add My Cell ──────────────│
  │     (cell ID assigned)        │
  │                               │
  │←═══ Game Loop Begins ═════════│
  │     (25Hz updates)            │
```

### 2. Game Loop

```
┌─────────────────────────────────────────┐
│            Server (40ms tick)           │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│  WebSocket Message (binary)             │
│  - Opcode (1 byte, shuffled)            │
│  - Payload (variable)                   │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│  Protocol.DecodeMessage()               │
│  1. Unshuffle opcode                    │
│  2. Parse based on opcode type          │
│  3. Return typed message                │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│  TUI Update()                           │
│  1. Type switch on message              │
│  2. Update game.World state             │
│  3. Return next Read() command          │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│  TUI View()                             │
│  1. Get visible cells from world        │
│  2. Transform world → screen coords     │
│  3. Render to buffer                    │
│  4. Return tea.View                     │
└─────────────────────────────────────────┘
```

### 3. Input Flow

```
User Input          Bubble Tea              Our Code
    │                    │                      │
    │ Mouse Move         │                      │
    │───────────────────→│                      │
    │                    │  tea.MouseMsg        │
    │                    │─────────────────────→│
    │                    │                      │
    │                    │                  Convert screen coords
    │                    │                  to world coords
    │                    │                      │
    │                    │                  Calculate delta
    │                    │                  from player center
    │                    │                      │
    │                    │                  EncodeMouseMove(dx, dy)
    │                    │                      │
    │                    │                  Send to server
    │                    │                      │
```

## Coordinate Systems

### World Coordinates
- Origin (0, 0) at center of map
- Map size: ±7071 on both axes (14142 × 14142 total)
- Cell positions and radii in world units

### Screen Coordinates  
- Origin (0, 0) at top-left of terminal
- Character cells, not pixels
- Size depends on terminal (e.g., 120 × 40)

### Camera Transform

```go
// World → Screen
screenX = (worldX - cameraX) * zoom + screenWidth/2
screenY = (worldY - cameraY) * zoom + screenHeight/2

// Screen → World  
worldX = cameraX + (screenX - screenWidth/2) / zoom
worldY = cameraY + (screenY - screenHeight/2) / zoom

// Radius → Screen size
screenRadius = worldRadius * zoom
```

### Zoom Calculation

Server sends zoom based on player mass. Approximate values:
- New spawn (radius 30): zoom ≈ 0.1
- Medium (radius 100): zoom ≈ 0.3
- Large (radius 300): zoom ≈ 0.6

## Protocol Details

### Opcode Shuffling

The server sends a 256-byte shuffle table during handshake. All opcodes are XOR'd with this table to obfuscate the protocol.

```go
// Encoding (client → server)
wireOpcode = shuffleTable[logicalOpcode]

// Decoding (server → client)  
logicalOpcode = inverseTable[wireOpcode]
```

### Message Types

| Opcode | Name | Direction | Description |
|--------|------|-----------|-------------|
| 0 | SPAWN | C→S | Spawn request (JSON) |
| 1 | MOUSE_MOVE | C→S | Movement direction (f32, f32) |
| 2 | SPLIT | C→S | Split request |
| 3 | EJECT | C→S | Eject mass |
| 220 | CAPTCHA | C→S | Captcha token (JSON) |
| 16 | WORLD_UPDATE | S→C | Cell positions |
| 17 | CAMERA | S→C | Camera x, y, zoom |
| 32 | ADD_MY_CELL | S→C | Cell ID assigned to player |
| 33 | REMOVE_MY_CELL | S→C | Cell ID removed from player |
| 49 | LEADERBOARD | S→C | Top players |
| 64 | BORDER | S→C | Map boundaries |
| 221 | SPAWN_RESULT | S→C | Spawn accepted/rejected |

### Binary Format

All multi-byte values are **little-endian**. Strings are **null-terminated UTF-8**.

#### World Update Format
```
[opcode: 1][eaten_count: 2][eaten_pairs: eaten_count*8][cells...]
                                                 
Each cell:
[id: 4][x: 4][y: 4][radius: 4][color: 1][flags: 1][name?: n][skin?: n]
                                                    ↑ null-terminated
```

## Rendering

### Cell Characters

```
Screen Radius    Character    Description
─────────────────────────────────────────
< 2              ·            Food, tiny cells
2-3              •            Small cells
4-7              ○            Medium cells  
8-11             ●            Large cells
≥ 12             ⬤            Huge cells

Player cells     ◉            Always distinct
```

### Rendering Algorithm

```
1. Clear buffer (fill with spaces)
2. For each visible cell (sorted by Y for layering):
   a. Transform world → screen coords
   b. Calculate screen radius
   c. Choose character based on size
   d. Draw filled circle to buffer
3. Draw HUD overlay (score, cells, controls)
4. Convert buffer to string
5. Return to Bubble Tea
```

## Bubble Tea v2 Integration

### Model

```go
type Model struct {
    world     *game.World      // Game state
    client    *client.Client   // WebSocket
    w, h      int              // Terminal size
    state     state            // Current state
    name      string           // Player name
}
```

### Update Pattern

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        // Handle keyboard
    case tea.MouseMsg:
        // Handle mouse, send to server
    case tea.WindowSizeMsg:
        // Update dimensions
    case client.ServerMsg:
        // Update world state
        return m, m.client.Read()  // Continue reading
    }
}
```

### View Pattern

```go
func (m Model) View() tea.View {
    content := m.renderGame()  // Or centered text
    
    v := tea.NewView(content)
    v.AltScreen = true              // Alternate screen buffer
    v.MouseMode = tea.MouseModeCellMotion  // Track mouse movement
    v.WindowTitle = "h4kmally-tui"
    return v
}
```

## Known Issues

1. **Camera zoom = 0**: Server sometimes sends 0, we default to 0.1
2. **Protocol decode errors**: Some cells have truncated data, we skip them
3. **No TTY in containers**: Requires `-test` flag or real terminal
4. **Flickering**: No double-buffering, might flicker on slow terminals

## Performance Considerations

- **Message rate**: ~25Hz from server (40ms tick)
- **Rendering**: Triggered by tick, not every message
- **Cell count**: Can be 100+, only render visible
- **Memory**: Cells stored in map, removed when server says so
