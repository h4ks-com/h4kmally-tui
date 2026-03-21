// Package tui implements the terminal UI
package tui

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"tui-agar/internal/client"
	"tui-agar/internal/game"
	"tui-agar/internal/protocol"
)

type state int

const (
	stateConnecting state = iota
	stateHandshake
	statePlaying
	stateDead
	stateError
)

var dbg *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/tui-internal.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		dbg = log.New(f, "", log.Lmicroseconds)
	}
}

type Model struct {
	world     *game.World
	client    *client.Client
	w, h      int
	state     state
	name      string
	err       error
	connected bool
	msgCount  int
	mouseScreenX int
	mouseScreenY int
}

func New(url, name string) Model {
	return Model{
		world:  game.NewWorld(),
		client: client.New(url),
		name:   name,
		state:  stateConnecting,
		w:      80,
		h:      24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.client.Connect(),
		tick(),
	)
}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type tickMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		cmds := []tea.Cmd{tick()}
		if m.state == statePlaying && m.w > 0 {
			wx, wy := m.world.ToWorld(float32(m.mouseScreenX), float32(m.mouseScreenY), float32(m.w), float32(m.h))
			cmds = append(cmds, m.client.Move(int32(wx), int32(wy)))
		}
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil

	case client.ConnectedMsg:
		m.connected = true
		m.state = stateHandshake
		return m, m.client.Read()

	case client.HandshakeDoneMsg:
		m.state = stateHandshake
		return m, tea.Batch(
			m.client.Captcha("skip"),
			m.client.Spawn(m.name),
			m.client.Read(),
		)

	case client.ServerMsg:
		m.msgCount++
		if dbg != nil && m.msgCount%100 == 0 {
			dbg.Printf("ServerMsg #%d: %T", m.msgCount, msg.Msg)
		}
		cmd := m.handleServer(msg)
		return m, tea.Batch(cmd, m.client.Read())

	case client.DisconnectedMsg:
		if m.err == nil && msg.Err != nil {
			m.err = msg.Err
		}
		m.state = stateError
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.client.Close()
		return m, tea.Quit
	case "space":
		if m.state == statePlaying {
			return m, m.client.Split()
		}
	case "w":
		if m.state == statePlaying {
			return m, m.client.Eject()
		}
	case "r":
		if m.state == stateDead {
			m.state = stateHandshake
			return m, tea.Batch(m.client.Spawn(m.name), m.client.Read())
		}
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.state != statePlaying || m.w == 0 {
		return m, nil
	}
	mouse := msg.Mouse()
	m.mouseScreenX, m.mouseScreenY = mouse.X, mouse.Y
	return m, nil
}

func (m *Model) handleServer(msg client.ServerMsg) tea.Cmd {
	switch v := msg.Msg.(type) {
	case protocol.BorderMsg:
		m.world.SetBorder(v.Left, v.Top, v.Right, v.Bottom)

	case protocol.CameraMsg:
		m.world.SetCamera(v.X, v.Y, v.Zoom)
		if m.state == stateHandshake {
			m.state = statePlaying
		}

	case protocol.WorldUpdateMsg:
		for _, c := range v.AddCells {
			m.world.AddCell(c.ID, c.X, c.Y, c.Size, c.R, c.G, c.B, c.Name, c.Skin)
		}
		for _, id := range v.RemoveIDs {
			m.world.RemoveCell(id)
		}
		if m.world.IsAlive() {
			m.state = statePlaying
		} else if m.state == statePlaying {
			m.state = stateDead
		}

	case protocol.AddMyCellMsg:
		m.world.AddMyCell(v.ID)
		if m.state == stateHandshake {
			m.state = statePlaying
		}

	case protocol.AddMultiCellMsg:
		// secondary multibox cell — ignore

	case protocol.SpawnResultMsg:
		if v.Accepted && m.state == stateHandshake {
			m.state = statePlaying
		}

	case protocol.ClearAllMsg:
		m.world.ClearAll()
		m.state = stateHandshake

	case protocol.ClearMineMsg:
		m.world.ClearMine()
		if m.state == statePlaying {
			m.state = stateDead
		}

	case protocol.LeaderboardMsg:
		m.world.Leaderboard = make([]game.LeaderEntry, len(v.Entries))
		for i, e := range v.Entries {
			m.world.Leaderboard[i] = game.LeaderEntry{
				Name:         e.Name,
				Rank:         e.Rank,
				IsMe:         e.IsMe,
				IsSubscriber: e.IsSubscriber,
			}
		}

	case protocol.PingReplyMsg:
		return m.client.Ping()
	}
	return nil
}

func (m Model) View() tea.View {
	var content string

	switch m.state {
	case stateConnecting:
		content = m.center("Connecting...")
	case stateHandshake:
		content = m.center(fmt.Sprintf("Spawning as %s...", m.name))
	case stateDead:
		content = m.center("You died!\n\nPress R to respawn")
	case stateError:
		e := "Disconnected"
		if m.err != nil {
			e = fmt.Sprintf("Error: %v", m.err)
		}
		content = m.center(e + "\n\nPress Q to quit")
	case statePlaying:
		content = m.renderGame()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = fmt.Sprintf("h4kmally-tui - %s", m.name)
	return v
}

func (m Model) center(text string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i := 0; i < (m.h-len(lines))/2; i++ {
		b.WriteByte('\n')
	}
	for _, l := range lines {
		lp := (m.w - len(l)) / 2
		if lp > 0 {
			b.WriteString(strings.Repeat(" ", lp))
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// pixel holds one terminal cell's content and color
type pixel struct {
	ch       rune
	r, g, b  uint8
	hasColor bool
	bold     bool
}

func (m Model) renderGame() string {
	w, h := m.w, m.h

	buf := make([][]pixel, h)
	for i := range buf {
		buf[i] = make([]pixel, w)
		for j := range buf[i] {
			buf[i][j] = pixel{ch: ' '}
		}
	}

	// Dynamic zoom: keeps player cell at a comfortable screen radius
	zoom := m.world.ViewZoom()

	visible := m.world.VisibleCellsZ(w, h, zoom)
	for _, c := range visible {
		sx := int((c.X-m.world.CamX)*zoom + float32(w)/2)
		sy := int((c.Y-m.world.CamY)*zoom + float32(h)/2)
		sr := int(c.Radius * zoom)
		if sr < 1 {
			sr = 1
		}
		ch := cellChar(sr, c.IsMine)
		for dy := -sr; dy <= sr; dy++ {
			for dx := -sr; dx <= sr; dx++ {
				if dx*dx+dy*dy <= sr*sr {
					px, py := sx+dx, sy+dy
					if py >= 0 && py < h && px >= 0 && px < w {
						buf[py][px] = pixel{ch: ch, r: c.R, g: c.G, b: c.B, hasColor: true}
					}
				}
			}
		}
	}

	// World border, then overlays
	m.stampBorder(buf, w, h, zoom)
	m.stampLeaderboard(buf, w)
	m.stampMinimap(buf, w)

	// HUD: score top-left, cells count
	writeHUD(buf, w, m.world.Score(), len(m.world.MyCells))

	// Help bar at bottom
	help := "[Mouse] [Space:Split] [W:Eject] [R:Respawn] [Q:Quit]"
	if len(help) < w {
		start := (w - len(help)) / 2
		for i, ch := range help {
			if start+i >= 0 && start+i < w && h > 1 {
				buf[h-1][start+i] = pixel{ch: ch}
			}
		}
	}

	var b strings.Builder
	for row := 0; row < h; row++ {
		var lastR, lastG, lastB uint8
		lastBold := false
		colorActive := false
		for _, px := range buf[row] {
			boldChanged := px.hasColor && px.bold != lastBold
			colorChanged := px.hasColor && (!colorActive || px.r != lastR || px.g != lastG || px.b != lastB)
			if colorActive && (!px.hasColor || boldChanged) {
				b.WriteString("\x1b[0m")
				colorActive = false
				lastBold = false
			}
			if px.hasColor && (colorChanged || boldChanged || !colorActive) {
				if px.bold {
					fmt.Fprintf(&b, "\x1b[1;38;2;%d;%d;%dm", px.r, px.g, px.b)
				} else {
					fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", px.r, px.g, px.b)
				}
				lastR, lastG, lastB = px.r, px.g, px.b
				lastBold = px.bold
				colorActive = true
			}
			b.WriteRune(px.ch)
		}
		if colorActive {
			b.WriteString("\x1b[0m")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// stampBorder draws the world boundary box into the pixel buffer.
func (m Model) stampBorder(buf [][]pixel, w, h int, zoom float32) {
	b := m.world.Border
	if b.Right <= b.Left || b.Bottom <= b.Top {
		return
	}

	const r, g, bl uint8 = 80, 80, 160 // dim blue-purple

	camX, camY := m.world.CamX, m.world.CamY
	fw, fh := float32(w), float32(h)

	leftX := int((float32(b.Left)-camX)*zoom + fw/2)
	rightX := int((float32(b.Right)-camX)*zoom + fw/2)
	topY := int((float32(b.Top)-camY)*zoom + fh/2)
	botY := int((float32(b.Bottom)-camY)*zoom + fh/2)

	// horizontal lines
	for _, sy := range []int{topY, botY} {
		if sy < 0 || sy >= h {
			continue
		}
		for x := 0; x < w; x++ {
			buf[sy][x] = pixel{ch: '─', r: r, g: g, b: bl, hasColor: true}
		}
	}
	// vertical lines
	for _, sx := range []int{leftX, rightX} {
		if sx < 0 || sx >= w {
			continue
		}
		for y := 0; y < h; y++ {
			buf[y][sx] = pixel{ch: '│', r: r, g: g, b: bl, hasColor: true}
		}
	}
	// corners
	corners := []struct {
		sy, sx int
		ch     rune
	}{
		{topY, leftX, '┌'}, {topY, rightX, '┐'},
		{botY, leftX, '└'}, {botY, rightX, '┘'},
	}
	for _, c := range corners {
		setPixel(buf, c.sy, c.sx, h, w, pixel{ch: c.ch, r: r, g: g, b: bl, hasColor: true})
	}
}

// stampLeaderboard overlays the leaderboard in the top-right corner of the buffer.
func (m Model) stampLeaderboard(buf [][]pixel, w int) {
	lb := m.world.Leaderboard
	if len(lb) == 0 {
		return
	}

	panelW := lbPanelWidth(lb)
	startCol := w - panelW
	if startCol < 0 {
		return
	}

	header := "Leaderboard"
	pad := (panelW - len(header)) / 2
	for i, ch := range strings.Repeat(" ", pad) + header {
		setPixel(buf, 0, startCol+i, m.h, w, pixel{ch: ch, r: 255, g: 255, b: 255, hasColor: true, bold: true})
	}
	for c := startCol + pad + len(header); c < w; c++ {
		setPixel(buf, 0, c, m.h, w, pixel{ch: ' ', r: 255, g: 255, b: 255, hasColor: true})
	}

	limit := len(lb)
	if limit > 10 {
		limit = 10
	}
	for i, e := range lb[:limit] {
		row := i + 1
		name := e.Name
		if name == "" {
			name = "(unnamed)"
		}
		line := fmt.Sprintf("%2d. %s", e.Rank, name)
		if len(line) > panelW {
			line = line[:panelW]
		}

		var r, g, b uint8
		bold := false
		if e.IsMe {
			r, g, b = 50, 220, 100
			bold = true
		} else if e.IsSubscriber {
			r, g, b = 200, 180, 0
		} else {
			r, g, b = 200, 200, 200
		}

		for j, ch := range line {
			setPixel(buf, row, startCol+j, m.h, w, pixel{ch: ch, r: r, g: g, b: b, hasColor: true, bold: bold})
		}
		for c := startCol + len(line); c < w; c++ {
			setPixel(buf, row, c, m.h, w, pixel{ch: ' ', r: r, g: g, b: b, hasColor: true})
		}
	}
}

// stampMinimap draws a 7×7 grid (A-G cols, 1-7 rows) in the bottom-right corner.
// The player's current quadrant is highlighted.
func (m Model) stampMinimap(buf [][]pixel, w int) {
	const cols = 7
	const rows = 7
	const cellW = 3 // "A1 " per cell
	const mmW = cols * cellW
	const mmH = rows + 1 // 1 header + 7 data rows

	if w < mmW+2 || m.h < mmH+2 {
		return
	}

	border := m.world.Border
	if border.Right <= border.Left || border.Bottom <= border.Top {
		return
	}

	// Player position in world → grid cell
	px, py := m.world.Center()
	gridCol := int((float64(px) - border.Left) / ((border.Right - border.Left) / float64(cols)))
	gridRow := int((float64(py) - border.Top) / ((border.Bottom - border.Top) / float64(rows)))
	if gridCol < 0 {
		gridCol = 0
	} else if gridCol > cols-1 {
		gridCol = cols - 1
	}
	if gridRow < 0 {
		gridRow = 0
	} else if gridRow > rows-1 {
		gridRow = rows - 1
	}

	// Placement: above help bar, right-aligned
	startCol := w - mmW - 1
	startRow := m.h - mmH - 1

	colLetters := [cols]byte{'A', 'B', 'C', 'D', 'E', 'F', 'G'}

	// Header row
	hdrRow := startRow
	for col := 0; col < cols; col++ {
		c := startCol + col*cellW
		setPixel(buf, hdrRow, c, m.h, w, pixel{ch: ' ', r: 120, g: 120, b: 120, hasColor: true})
		setPixel(buf, hdrRow, c+1, m.h, w, pixel{ch: rune(colLetters[col]), r: 120, g: 120, b: 120, hasColor: true})
		setPixel(buf, hdrRow, c+2, m.h, w, pixel{ch: ' ', r: 120, g: 120, b: 120, hasColor: true})
	}

	// Data rows
	for row := 0; row < rows; row++ {
		r := startRow + 1 + row
		for col := 0; col < cols; col++ {
			c := startCol + col*cellW
			isMe := col == gridCol && row == gridRow

			var fr, fg, fb uint8
			if isMe {
				fr, fg, fb = 50, 220, 100
			} else {
				fr, fg, fb = 80, 80, 80
			}

			letter := rune(colLetters[col])
			digit := rune('1' + row)

			setPixel(buf, r, c, m.h, w, pixel{ch: letter, r: fr, g: fg, b: fb, hasColor: true})
			setPixel(buf, r, c+1, m.h, w, pixel{ch: digit, r: fr, g: fg, b: fb, hasColor: true})
			setPixel(buf, r, c+2, m.h, w, pixel{ch: ' ', r: fr, g: fg, b: fb, hasColor: true})
		}
	}
}

func setPixel(buf [][]pixel, row, col, h, w int, p pixel) {
	if row >= 0 && row < h && col >= 0 && col < w {
		buf[row][col] = p
	}
}

func cellChar(sr int, isMine bool) rune {
	if isMine {
		return '◉'
	}
	switch {
	case sr < 2:
		return '·'
	case sr < 4:
		return '•'
	case sr < 8:
		return '○'
	case sr >= 12:
		return '⬤'
	default:
		return '●'
	}
}

func writeHUD(buf [][]pixel, w, score, cells int) {
	if len(buf) == 0 {
		return
	}
	row := buf[0]
	hud := fmt.Sprintf("Score: %d  Cells: %d", score, cells)
	for i, ch := range hud {
		if i < w {
			row[i] = pixel{ch: ch}
		}
	}
}

func lbPanelWidth(lb []game.LeaderEntry) int {
	maxName := 8 // "Leaderboard" header minus prefix
	for _, e := range lb {
		n := len(e.Name)
		if n > maxName {
			maxName = n
		}
	}
	// "N. name" + 2 padding
	return maxName + 6
}

