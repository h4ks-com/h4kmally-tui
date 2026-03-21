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

// charAspect is the pixel height-to-width ratio of a typical monospace terminal
// character (~2:1). Used to correct the circle rendering and mouse aim so that
// cells appear round and horizontal/vertical movement feels equal speed.
const charAspect = 2

var dbg *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/tui-internal.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		dbg = log.New(f, "", log.Lmicroseconds)
	}
}

type Model struct {
	world        *game.World
	client       *client.Client
	w, h         int
	state        state
	name         string
	err          error
	connected    bool
	msgCount     int
	mouseScreenX int
	mouseScreenY int
	chatMode     bool
	chatInput    string
	chatMessages []ChatMessage
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
			vz := m.world.ViewZoom()
			wx := m.world.CamX + (float32(m.mouseScreenX)-float32(m.w)/2)/vz
			wy := m.world.CamY + (float32(m.mouseScreenY)-float32(m.h)/2)*charAspect/vz
			cmds = append(cmds, m.client.Move(int32(wx), int32(wy)))
		}
		now := time.Now()
		n := 0
		for _, cm := range m.chatMessages {
			if now.Sub(cm.At) < time.Minute {
				m.chatMessages[n] = cm
				n++
			}
		}
		m.chatMessages = m.chatMessages[:n]
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

// handleKey dispatches to the active context: chat or game.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.chatMode {
		return m.handleChatKey(msg.String())
	}
	return m.handleGameKey(msg.String())
}

// handleChatKey runs while chat mode is active.
// ALL keys are consumed here — game shortcuts have zero effect.
// Only "enter" (send + exit) and "esc" (discard + exit) leave chat mode.
func (m Model) handleChatKey(key string) (Model, tea.Cmd) {
	switch key {
	case "enter":
		text := m.chatInput
		m.chatInput = ""
		m.chatMode = false
		if text != "" && m.client != nil {
			return m, m.client.Chat(text)
		}
		return m, nil
	case "esc":
		m.chatInput = ""
		m.chatMode = false
		return m, nil
	case "backspace":
		if len(m.chatInput) > 0 {
			m.chatInput = m.chatInput[:len(m.chatInput)-1]
		}
		return m, nil
	case "space":
		// bubbletea names the spacebar "space", not " "
		if len(m.chatInput) < 100 {
			m.chatInput += " "
		}
		return m, nil
	default:
		// Accept single printable ASCII characters; ignore named keys
		// (arrows, F-keys, ctrl+*, etc.) which have multi-char string forms.
		if len(key) == 1 && key[0] >= 32 && len(m.chatInput) < 100 {
			m.chatInput += key
		}
		return m, nil
	}
}

// handleGameKey runs while chat mode is inactive (normal gameplay).
func (m Model) handleGameKey(key string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c", "q":
		if m.client != nil {
			m.client.Close()
		}
		return m, tea.Quit
	case "space":
		if m.state == statePlaying && m.client != nil {
			return m, m.client.Split()
		}
	case "w":
		if m.state == statePlaying && m.client != nil {
			return m, m.client.Eject()
		}
	case "r":
		if m.state == stateDead && m.client != nil {
			m.state = stateHandshake
			return m, tea.Batch(m.client.Spawn(m.name), m.client.Read())
		}
	case "enter":
		if m.state == statePlaying {
			m.chatMode = true
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
			m.world.AddCell(c.ID, c.X, c.Y, c.Size, c.R, c.G, c.B, c.Name, c.Skin, c.IsVirus)
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

	case protocol.ChatMsg:
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Name: v.Name, R: v.R, G: v.G, B: v.B,
			Text: v.Text, At: time.Now(),
		})

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

// ChatMessage holds a received chat message with display metadata
type ChatMessage struct {
	Name    string
	R, G, B uint8
	Text    string
	At      time.Time
}

// pixel holds one terminal cell's content and color
type pixel struct {
	ch            rune
	r, g, b       uint8
	bgR, bgG, bgB uint8
	hasColor      bool
	hasBg         bool
	bold          bool
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

	// charAspect correction: fetch cells from a 2× taller frustum so cells
	// near the top/bottom edge are not culled after Y-coordinate correction.
	visible := m.world.VisibleCellsZ(w, h*charAspect, zoom)
	for _, c := range visible {
		sx := int((c.X-m.world.CamX)*zoom + float32(w)/2)
		// Divide Y by charAspect so 1 world unit maps to the same visual
		// distance in both axes (chars are ~2× taller than wide).
		sy := int((c.Y-m.world.CamY)*zoom/charAspect + float32(h)/2)
		sr := int(c.Radius * zoom)
		if sr < 1 {
			sr = 1
		}
		cr, cg, cb := c.R, c.G, c.B
		if !c.IsVirus {
			cr, cg, cb = avoidVirusGreen(cr, cg, cb)
		}
		ch := cellChar(sr, c.IsMine)
		// Ellipse: dx²+ (charAspect·dy)² ≤ sr² renders as a visual circle.
		halfSr := (sr + charAspect - 1) / charAspect
		for dy := -halfSr; dy <= halfSr; dy++ {
			for dx := -sr; dx <= sr; dx++ {
				if dx*dx+charAspect*charAspect*dy*dy <= sr*sr {
					px, py := sx+dx, sy+dy
					if py >= 0 && py < h && px >= 0 && px < w {
						buf[py][px] = pixel{ch: ch, r: cr, g: cg, b: cb, hasColor: true}
					}
				}
			}
		}
	}

	// Stamp cell names on top of filled circles
	for _, c := range visible {
		if c.Name == "" {
			continue
		}
		sx := int((c.X-m.world.CamX)*zoom + float32(w)/2)
		sy := int((c.Y-m.world.CamY)*zoom/charAspect + float32(h)/2)
		sr := int(c.Radius * zoom)
		if sr < 3 {
			continue
		}
		name := c.Name
		maxLen := 2*sr - 1
		if len(name) > maxLen {
			name = name[:maxLen]
		}
		label := " " + name + " "
		startX := sx - len(label)/2
		for i, ch := range label {
			nx, ny := startX+i, sy
			if ny >= 0 && ny < h && nx >= 0 && nx < w {
				buf[ny][nx] = pixel{
					ch: ch, r: 255, g: 255, b: 255, hasColor: true, bold: true,
					bgR: 0, bgG: 0, bgB: 0, hasBg: true,
				}
			}
		}
	}

	// World border, then overlays
	m.stampBorder(buf, w, h, zoom)
	m.stampLeaderboard(buf, w)
	m.stampMinimap(buf, w)
	m.stampChat(buf, w)

	// HUD: score top-left, cells count
	writeHUD(buf, w, m.world.Score(), len(m.world.MyCells))

	// Help bar at bottom
	help := "[Mouse] [Space:Split] [W:Eject] [R:Respawn] [Enter:Chat] [Q:Quit]"
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
		var lastBgR, lastBgG, lastBgB uint8
		lastBold := false
		colorActive := false
		bgActive := false
		for _, px := range buf[row] {
			boldChanged := px.hasColor && px.bold != lastBold
			if (colorActive && !px.hasColor) || (bgActive && !px.hasBg) || boldChanged {
				b.WriteString("\x1b[0m")
				colorActive = false
				bgActive = false
				lastBold = false
			}
			if px.hasBg && (!bgActive || px.bgR != lastBgR || px.bgG != lastBgG || px.bgB != lastBgB) {
				fmt.Fprintf(&b, "\x1b[48;2;%d;%d;%dm", px.bgR, px.bgG, px.bgB)
				lastBgR, lastBgG, lastBgB = px.bgR, px.bgG, px.bgB
				bgActive = true
			}
			if px.hasColor && (!colorActive || px.r != lastR || px.g != lastG || px.b != lastB || px.bold != lastBold) {
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
		if colorActive || bgActive {
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
	topY := int((float32(b.Top)-camY)*zoom/charAspect + fh/2)
	botY := int((float32(b.Bottom)-camY)*zoom/charAspect + fh/2)

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
		switch {
		case e.IsMe:
			r, g, b = 50, 220, 100
			bold = true
		case e.IsSubscriber:
			r, g, b = 200, 180, 0
		default:
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

// stampChat draws the chat history and input row in the bottom-left corner.
func (m Model) stampChat(buf [][]pixel, w int) {
	const maxMsgs = 5
	const chatW = 48

	now := time.Now()
	recent := make([]ChatMessage, 0, maxMsgs)
	for _, cm := range m.chatMessages {
		if now.Sub(cm.At) < time.Minute {
			recent = append(recent, cm)
		}
	}
	if len(recent) > maxMsgs {
		recent = recent[len(recent)-maxMsgs:]
	}

	// Row above the help bar is reserved for the input hint / active input
	hintRow := m.h - 2
	msgsStartRow := hintRow - len(recent)

	for i, cm := range recent {
		row := msgsStartRow + i
		if row < 0 || row >= m.h {
			continue
		}
		nameLabel := cm.Name + ": "
		full := nameLabel + cm.Text
		if len(full) > chatW {
			full = full[:chatW]
		}
		for j, ch := range full {
			var r, g, b uint8
			bold := false
			if j < len(nameLabel) {
				r, g, b = cm.R, cm.G, cm.B
				bold = true
			} else {
				r, g, b = 210, 210, 210
			}
			setPixel(buf, row, j, m.h, w, pixel{ch: ch, r: r, g: g, b: b, hasColor: true, bold: bold})
		}
	}

	if m.chatMode {
		input := "> " + m.chatInput + "█"
		if len(input) > chatW {
			input = input[:chatW]
		}
		for i, ch := range input {
			setPixel(buf, hintRow, i, m.h, w, pixel{
				ch: ch, r: 255, g: 220, b: 80, hasColor: true, bold: true,
				bgR: 15, bgG: 15, bgB: 15, hasBg: true,
			})
		}
	} else {
		hint := "↵ chat"
		for i, ch := range hint {
			setPixel(buf, hintRow, i, m.h, w, pixel{ch: ch, r: 70, g: 70, b: 70, hasColor: true})
		}
	}
}

// avoidVirusGreen remaps green-dominant colors on non-virus cells.
// Viruses are always green, so player/food cells that happen to have a
// green-dominant server color would be visually indistinguishable from viruses.
// A 120° hue rotation (R,G,B)→(B,R,G) moves green→blue while preserving saturation.
func avoidVirusGreen(r, g, b uint8) (uint8, uint8, uint8) {
	if int(g) > int(r)+60 && int(g) > int(b)+60 && g > 100 {
		return b, r, g
	}
	return r, g, b
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
