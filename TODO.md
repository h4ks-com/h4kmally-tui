# TODO & Research

## Research & References

### Protocol Documentation
- **Source**: https://github.com/h4ks-com/h4kmally-server/blob/main/PROTOCOL.md
- **What it covers**: Binary format, opcodes, message structures
- **What's missing**: Captcha flow, exact spawn sequence, skin loading

### Bubble Tea v2
- **Docs**: https://charm.land/bubbletea/v2
- **Upgrade Guide**: https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md
- **Key changes from v1**:
  - `View() string` → `View() tea.View`
  - `tea.KeyMsg` struct → `tea.KeyPressMsg`
  - `tea.MouseMsg` struct → `tea.MouseMsg` interface (call `.Mouse()`)
  - Commands like `tea.EnterAltScreen` → `view.AltScreen = true`

### Terminal Game References
- **Pokete**: https://github.com/lxgr-linux/pokete - Pokemon-like terminal game
  - Uses: Python + urwid
  - Takeaway: Sprite-based rendering, good for complex graphics
  
- **Gorched**: https://github.com/challenge-run/gorched - Scorched Earth clone
  - Uses: Go + termloop
  - Takeaway: Real-time physics in terminal

### Coordinate Transform Math
- Agar.io uses: World center at (0,0), camera follows player centroid
- Zoom formula: `screenRadius = worldRadius * zoom * scaleFactor`
- Our scale factor: 0.5 (terminal chars are roughly 2x tall as wide)

### Unicode Characters
- Source: https://unicode.org/charts/
- Selected: `· • ○ ● ⬤ ◉` (Geometric Shapes, Miscellaneous Symbols)
- Considered but rejected:
  - `░ ▒ ▓ █` (shading) - hard to make circles
  - `○ ◎ ●` (circles) - good, used
  - Emojis - inconsistent widths across terminals

## Implementation Status

### ✅ Completed

#### Protocol Layer
- [x] Handshake with shuffle table
- [x] Opcode shuffling/unshuffling
- [x] Decode: Border, Camera, WorldUpdate
- [x] Decode: AddMyCell, RemoveMyCell, SpawnResult
- [x] Encode: Spawn, MouseMove, Split, Eject, Captcha
- [x] Bounds checking on cell decoder
- [x] Error recovery on malformed messages

#### WebSocket Client
- [x] Dial with timeout
- [x] TLS support (wss://)
- [x] Read loop in goroutine
- [x] Message channel to Bubble Tea
- [x] Thread-safe send with mutex
- [x] Graceful close handling

#### Game Engine
- [x] World state container
- [x] Cell map (add/update/remove)
- [x] Player cell tracking
- [x] Camera position and zoom
- [x] Coordinate transforms (world ↔ screen)
- [x] Visible cell culling
- [x] Player center calculation
- [x] Score calculation

#### TUI Layer
- [x] Bubble Tea v2 integration
- [x] State machine (connecting → playing → dead)
- [x] Keyboard input (Q, Space, W, R)
- [x] Mouse input handling
- [x] Window size tracking
- [x] Alt screen buffer
- [x] Mouse cell motion mode
- [x] Basic HUD (score, cells, controls)
- [x] Cell rendering with Unicode
- [x] Centered text for states
- [x] Tick-based rendering loop

#### Build & Deploy
- [x] Go module setup
- [x] Dockerfile for client
- [x] Docker Compose for server
- [x] Makefile with common tasks
- [x] Test client for debugging
- [x] Debug logging to file

### 🚧 Partially Done

#### Mouse Movement
- [x] Captures mouse position
- [x] Converts to world coords
- [x] Calculates delta from center
- [x] Sends to server
- [ ] **UNTESTED**: Does player actually move?
- [ ] Need: Visual feedback that mouse is working

#### Cell Rendering  
- [x] Transform world → screen
- [x] Choose character by size
- [x] Draw filled circles
- [ ] **ISSUE**: May not see cells if zoom is wrong
- [ ] Need: Debug overlay showing zoom, player pos, cell count

#### Connection Flow
- [x] Connects to local server
- [x] Connects to wss://api.sigmally.h4ks.com/ws/
- [x] Handshake succeeds
- [x] Spawn succeeds
- [ ] **ISSUE**: TUI exits immediately on real terminal
- [ ] Need: Better error reporting, keep-alive

### ❌ Not Implemented

#### Features
- [ ] **Leaderboard** - Protocol decoded, no UI
- [ ] **Chat** - Not implemented
- [ ] **Minimap** - Not implemented
- [ ] **Skins** - Protocol supports, not rendered
- [ ] **Names above cells** - Protocol has names, not rendered
- [ ] **Cell colors** - Protocol has colors, not used
- [ ] **Split animation** - Instant split
- [ ] **Eat animation** - Instant eat
- [ ] **Death screen** - Basic text only
- [ ] **Respawn flow** - Basic, no countdown

#### UI Polish
- [ ] **Double buffering** - Prevents flicker
- [ ] **ANSI colors** - 256-color support
- [ ] **Cell glow effects** - Via color intensity
- [ ] **Smooth camera** - Interpolate between positions
- [ ] **FPS counter** - Debug info
- [ ] **Ping indicator** - Server latency
- [ ] **Better HUD** - More stats, nicer layout

#### Protocol
- [ ] **Reconnection** - Auto-retry on disconnect
- [ ] **Compression** - Server may support
- [ ] **Protocol versioning** - Handle different versions
- [ ] **Captcha actual solving** - Currently skips

#### Testing
- [ ] **Unit tests** - protocol package
- [ ] **Unit tests** - game package  
- [ ] **Integration tests** - Full flow
- [ ] **Visual tests** - Snapshot rendering

## Current Issues

### Critical: TUI Not Displaying

**Symptom**: When running in real terminal, TUI flickers and exits

**Debug info**:
```
Test mode works (sees "Score: 0, Cells: 1")
Protocol works (500+ messages in 10s)
Connection works (wss://api.sigmally.h4ks.com/ws/)
```

**Possible causes**:
1. **TTY access**: Bubble Tea can't open /dev/tty
2. **Immediate quit**: Something sending quit signal
3. **Window size**: Getting 0x0 size and exiting
4. **Error on first update**: Crash before first render

**Next debug steps**:
- [ ] Add more logging in Update()
- [ ] Log every message type received
- [ ] Check if Quit is being called unexpectedly
- [ ] Try with `tea.WithInput(os.Stdin)` explicitly
- [ ] Try with `tea.WithOutput(os.Stderr)` to see errors

### Medium: No Visual Feedback

**Symptom**: Can't tell if game is actually working

**Solution**: Add debug overlay
```
┌────────────────────────────────────────┐
│ Score: 0                    Cells: 1   │
│ Zoom: 0.10  Pos: 0,0   Visible: 5     │
│ MSG: WORLD_UPDATE                       │
│                                        │
│              ◉                         │
│                                        │
└────────────────────────────────────────┘
```

### Low: Protocol Decoding Incomplete

Some world updates may have truncated cells. Currently we skip them silently. Should log when this happens.

## Next Steps

### Immediate (to get playable)

1. **Fix TUI display issue**
   - Add extensive logging
   - Try different Bubble Tea options
   - Test with simpler rendering first
   
2. **Add debug overlay**
   - Show zoom, position, visible cells
   - Show last message type
   - Show connection state

3. **Test mouse input visually**
   - Add cursor position to HUD
   - Show direction arrow toward mouse
   - Verify player moves

### Short-term (make it good)

4. **Add colors**
   - Map cell color byte to ANSI
   - Different colors for player vs others
   - Food in one color, players in another

5. **Add names**
   - Render cell names above them
   - Truncate long names
   - Color by team/clan

6. **Add leaderboard**
   - Protocol already decoded
   - Show top 10 in corner
   - Highlight self if on board

### Long-term (make it great)

7. **Minimap**
   - Show whole world in corner
   - Player position indicator
   - Other cells as dots

8. **Smooth camera**
   - Interpolate camera position
   - Smooth zoom transitions
   - Follow player with lag

9. **Chat system**
   - Protocol research needed
   - Input field overlay
   - Message display

10. **Reconnection**
    - Auto-retry on disconnect
    - Preserve player name
    - Show reconnecting state

## Testing Checklist

### Local Server
```bash
cd h4kmally-server
PORT=3001 ./server-bin

# In another terminal:
cd h4kmally-tui
go run ./cmd/tui-agar -name TestBot
```
- [ ] Connects successfully
- [ ] Sees own cell
- [ ] Mouse moves cell
- [ ] Split works (Space)
- [ ] Eject works (W)
- [ ] Dies when hitting virus
- [ ] Respawn works (R)
- [ ] Quit works (Q)

### Public Server
```bash
go run ./cmd/tui-agar -name TestBot -server wss://api.sigmally.h4ks.com/ws/
```
- [ ] TLS connection works
- [ ] Spawns successfully
- [ ] Sees other players
- [ ] Can eat food
- [ ] Can eat smaller players
- [ ] Gets eaten by larger players

### Test Mode
```bash
go run ./cmd/tui-agar -test -name TestBot -server wss://api.sigmally.h4ks.com/ws/
```
- [ ] No TTY required
- [ ] Logs to /tmp/agar-debug.log
- [ ] Shows ANSI output to stderr
- [ ] Runs for 10 seconds
- [ ] No panics

## Performance Targets

- **Startup**: < 1 second to show game
- **Latency**: < 50ms from mouse to movement
- **Rendering**: 30 FPS minimum
- **Memory**: < 50 MB with 200 cells
- **CPU**: < 10% single core

## Success Criteria

**Minimum Viable**:
- Can connect to server
- Can see own cell
- Can move with mouse
- Doesn't crash

**Good**:
- Can see other players
- Can eat food
- Has colors
- Has names

**Great**:
- Has leaderboard
- Has minimap
- Smooth camera
- Chat system

**Amazing**:
- Skins
- Particles
- Sound
- Themes
