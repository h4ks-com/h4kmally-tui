// Package tui implements the terminal UI
package tui

import (
	"fmt"
	"strings"

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

// Model is the app state
type Model struct {
	world  *game.World
	client *client.Client
	w, h   int
	state  state
	name   string
	err    error
}

// New creates the model
func New(url, name string) Model {
	return Model{
		world:  game.NewWorld(),
		client: client.New(url),
		name:   name,
	}
}

// Init starts the app
func (m Model) Init() tea.Cmd { return m.client.Connect() }

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case client.ConnectedMsg:
		m.state = stateHandshake
		return m, m.client.Read()
	case client.HandshakeDoneMsg:
		return m, tea.Batch(m.client.Captcha("skip"), m.client.Spawn(m.name), m.client.Read())
	case client.ServerMsg:
		return m.handleServer(msg)
	case client.DisconnectedMsg:
		m.err = msg.Err
		m.state = stateError
		return m, tea.Quit
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
	wx, wy := m.world.ToWorld(float32(mouse.X), float32(mouse.Y), float32(m.w), float32(m.h))
	px, py := m.world.Center()
	return m, m.client.Move(wx-px, wy-py)
}

func (m Model) handleServer(msg client.ServerMsg) (tea.Model, tea.Cmd) {
	switch v := msg.Msg.(type) {
	case protocol.BorderMsg:
		m.world.SetBorder(v.Left, v.Top, v.Right, v.Bottom)
	case protocol.CameraMsg:
		m.world.SetCamera(v.X, v.Y, v.Zoom)
	case protocol.WorldUpdateMsg:
		for _, c := range v.AddCells {
			m.world.AddCell(c.ID, c.X, c.Y, c.Radius, c.Color, c.Name, c.Skin)
		}
		for _, id := range v.RemoveIDs {
			m.world.RemoveCell(id)
		}
		if m.world.IsAlive() {
			m.state = statePlaying
		}
	case uint32:
		m.world.AddMyCell(v)
		m.state = statePlaying
	case protocol.SpawnResultMsg:
		if v.Accepted {
			m.state = statePlaying
		}
	}
	return m, m.client.Read()
}

// View renders the UI
func (m Model) View() tea.View {
	if m.w == 0 {
		return tea.NewView("Loading...")
	}
	var content string
	switch m.state {
	case stateConnecting:
		content = m.center("Connecting...")
	case stateHandshake:
		content = m.center("Spawning...")
	case stateDead:
		content = m.center("You died!\n\nPress R to respawn")
	case stateError:
		e := "Disconnected"
		if m.err != nil {
			e = fmt.Sprintf("Error: %v", m.err)
		}
		content = m.center(e)
	case statePlaying:
		content = m.renderGame()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "tui-agar"
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

func (m Model) renderGame() string {
	buf := make([][]rune, m.h)
	for i := range buf {
		buf[i] = make([]rune, m.w)
		for j := range buf[i] {
			buf[i][j] = ' '
		}
	}
	for _, c := range m.world.VisibleCells(m.w, m.h) {
		m.renderCell(buf, c)
	}
	// HUD
	score := fmt.Sprintf("Score: %d", m.world.Score())
	for i, ch := range score {
		if i < m.w {
			buf[0][i] = ch
		}
	}
	cells := fmt.Sprintf("Cells: %d", len(m.world.MyCells))
	start := m.w - len(cells) - 1
	for i, ch := range cells {
		if start+i >= 0 && start+i < m.w {
			buf[0][start+i] = ch
		}
	}
	help := "[Mouse:Move] [Space:Split] [W:Eject] [Q:Quit]"
	start = (m.w - len(help)) / 2
	for i, ch := range help {
		if start+i >= 0 && start+i < m.w && m.h > 1 {
			buf[m.h-1][start+i] = ch
		}
	}
	var b strings.Builder
	for _, row := range buf {
		b.WriteString(string(row))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) renderCell(buf [][]rune, c *game.Cell) {
	sx, sy := m.world.ToScreen(c.X, c.Y, float32(m.w), float32(m.h))
	sr := m.world.ScreenRadius(c.Radius)
	if sr < 1 {
		sr = 1
	}
	ch := '●'
	if sr < 2 {
		ch = '·'
	} else if sr < 4 {
		ch = '•'
	} else if sr < 8 {
		ch = '○'
	} else if sr >= 12 {
		ch = '⬤'
	}
	if c.IsMine {
		ch = '◉'
	}
	for dy := -sr; dy <= sr; dy++ {
		for dx := -sr; dx <= sr; dx++ {
			if dx*dx+dy*dy <= sr*sr {
				px, py := sx+dx, sy+dy
				if py >= 0 && py < m.h && px >= 0 && px < m.w {
					buf[py][px] = ch
				}
			}
		}
	}
}
