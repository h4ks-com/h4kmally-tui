// Package game implements the game state and logic
package game

import (
	"math"
	"sort"
)

// Border represents world boundaries
type Border struct{ Left, Top, Right, Bottom float64 }

// Cell represents a cell in the world
type Cell struct {
	ID      uint32
	X, Y    float32
	Radius  float32
	R, G, B uint8
	Name    string
	Skin    string
	IsMine  bool
	IsVirus bool
}

// LeaderEntry is one row on the leaderboard
type LeaderEntry struct {
	Name         string
	Rank         uint32
	IsMe         bool
	IsSubscriber bool
}

// World holds game state
type World struct {
	Border      Border
	Cells       map[uint32]*Cell
	MyCells     []uint32
	CamX        float32
	CamY        float32
	CamZoom     float32
	Leaderboard []LeaderEntry
}

// NewWorld creates a new world
func NewWorld() *World {
	return &World{
		Cells:   make(map[uint32]*Cell),
		MyCells: make([]uint32, 0),
		CamZoom: 0.1,
	}
}

// AddCell adds or updates a cell from the new protocol format
func (w *World) AddCell(id uint32, x, y int16, size uint16, r, g, b uint8, name, skin string, isVirus bool) {
	fx, fy := float32(x), float32(y)
	radius := float32(size)
	if c, ok := w.Cells[id]; ok {
		c.X, c.Y, c.Radius = fx, fy, radius
		c.R, c.G, c.B = r, g, b
		c.IsVirus = isVirus
		if name != "" {
			c.Name = name
		}
	} else {
		w.Cells[id] = &Cell{ID: id, X: fx, Y: fy, Radius: radius, R: r, G: g, B: b, Name: name, Skin: skin, IsVirus: isVirus}
	}
}

// RemoveCell removes a cell
func (w *World) RemoveCell(id uint32) {
	delete(w.Cells, id)
	for i, mid := range w.MyCells {
		if mid == id {
			w.MyCells = append(w.MyCells[:i], w.MyCells[i+1:]...)
			break
		}
	}
}

// AddMyCell marks a cell as player's
func (w *World) AddMyCell(id uint32) {
	for _, e := range w.MyCells {
		if e == id {
			return
		}
	}
	w.MyCells = append(w.MyCells, id)
	if c, ok := w.Cells[id]; ok {
		c.IsMine = true
	} else {
		w.Cells[id] = &Cell{ID: id, IsMine: true}
	}
}

// ClearAll removes all cells
func (w *World) ClearAll() {
	w.Cells = make(map[uint32]*Cell)
	w.MyCells = w.MyCells[:0]
}

// ClearMine removes player's cells (death)
func (w *World) ClearMine() {
	for _, id := range w.MyCells {
		delete(w.Cells, id)
	}
	w.MyCells = w.MyCells[:0]
}

// SetCamera updates camera
func (w *World) SetCamera(x, y, zoom float32) {
	w.CamX, w.CamY = x, y
	if zoom > 0.001 {
		w.CamZoom = zoom
	}
	// zoom=0 from server (unused) — keep current zoom
}

// SetBorder updates boundaries
func (w *World) SetBorder(l, t, r, b float64) {
	w.Border = Border{l, t, r, b}
}

// Center returns player center (mass-weighted)
func (w *World) Center() (float32, float32) {
	if len(w.MyCells) == 0 {
		return w.CamX, w.CamY
	}
	var mass, cx, cy float32
	for _, id := range w.MyCells {
		if c, ok := w.Cells[id]; ok {
			m := c.Radius * c.Radius
			cx += c.X * m
			cy += c.Y * m
			mass += m
		}
	}
	if mass > 0 {
		return cx / mass, cy / mass
	}
	return w.CamX, w.CamY
}

// Score returns total mass
func (w *World) Score() int {
	var t int
	for _, id := range w.MyCells {
		if c, ok := w.Cells[id]; ok {
			r := c.Radius
			t += int(r * r / 100)
		}
	}
	return t
}

// IsAlive returns true if player has cells
func (w *World) IsAlive() bool { return len(w.MyCells) > 0 }

// ToScreen converts world to screen coords
func (w *World) ToScreen(wx, wy, sw, sh float32) (int, int) {
	return int((wx-w.CamX)*w.CamZoom + sw/2), int((wy-w.CamY)*w.CamZoom + sh/2)
}

// ToWorld converts screen to world coords
func (w *World) ToWorld(sx, sy, sw, sh float32) (float32, float32) {
	return w.CamX + (sx-sw/2)/w.CamZoom, w.CamY + (sy-sh/2)/w.CamZoom
}

// ScreenRadius converts radius to screen size
func (w *World) ScreenRadius(r float32) int { return int(r * w.CamZoom) }

// ViewZoom returns a zoom level based on player cell size.
// Uses sqrt scaling so the cell visibly grows on screen as mass increases,
// while still zooming out gradually. The server always sends zoom=0.
func (w *World) ViewZoom() float32 {
	var maxRadius float32
	for _, id := range w.MyCells {
		if c, ok := w.Cells[id]; ok && c.Radius > maxRadius {
			maxRadius = c.Radius
		}
	}
	if maxRadius <= 0 {
		return 0.05
	}
	// z = 0.7 / sqrt(radius): screen_radius = z*radius = 0.7*sqrt(radius)
	// grows as you eat (e.g. radius 50→5 chars, 200→10 chars, 800→20 chars)
	z := float32(0.7 / math.Sqrt(float64(maxRadius)))
	if z < 0.008 {
		z = 0.008
	}
	if z > 0.15 {
		z = 0.15
	}
	return z
}

// VisibleCellsZ returns cells visible with an explicit zoom, sorted small→large.
func (w *World) VisibleCellsZ(sw, sh int, zoom float32) []*Cell {
	v := make([]*Cell, 0, len(w.Cells))
	fsw, fsh := float32(sw), float32(sh)
	hw, hh := fsw/(2*zoom), fsh/(2*zoom)
	for _, c := range w.Cells {
		if c.X+c.Radius >= w.CamX-hw && c.X-c.Radius <= w.CamX+hw &&
			c.Y+c.Radius >= w.CamY-hh && c.Y-c.Radius <= w.CamY+hh {
			v = append(v, c)
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].Radius < v[j].Radius })
	return v
}

// VisibleCells returns cells in view sorted by size (small behind large)
func (w *World) VisibleCells(sw, sh int) []*Cell {
	return w.VisibleCellsZ(sw, sh, w.CamZoom)
}

// Distance between two points
func Distance(x1, y1, x2, y2 float32) float32 {
	dx, dy := x2-x1, y2-y1
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}
