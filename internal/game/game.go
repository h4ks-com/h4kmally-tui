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
	ID     uint32
	X, Y   float32
	Radius float32
	Color  uint8
	Name   string
	Skin   string
	IsMine bool
}

// World holds game state
type World struct {
	Border  Border
	Cells   map[uint32]*Cell
	MyCells []uint32
	CamX    float32
	CamY    float32
	CamZoom float32
}

// NewWorld creates a new world
func NewWorld() *World {
	return &World{
		Cells:   make(map[uint32]*Cell),
		MyCells: make([]uint32, 0),
		CamZoom: 0.1,
	}
}

// AddCell adds or updates a cell
func (w *World) AddCell(id uint32, x, y, radius float32, color uint8, name, skin string) {
	if c, ok := w.Cells[id]; ok {
		c.X, c.Y, c.Radius, c.Color = x, y, radius, color
		if name != "" {
			c.Name = name
		}
	} else {
		w.Cells[id] = &Cell{ID: id, X: x, Y: y, Radius: radius, Color: color, Name: name, Skin: skin}
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
	}
}

// RemoveMyCell removes player ownership
func (w *World) RemoveMyCell(id uint32) {
	for i, e := range w.MyCells {
		if e == id {
			w.MyCells = append(w.MyCells[:i], w.MyCells[i+1:]...)
			break
		}
	}
}

// SetCamera updates camera
func (w *World) SetCamera(x, y, zoom float32) {
	w.CamX, w.CamY = x, y
	if zoom > 0.001 {
		w.CamZoom = zoom
	} else {
		w.CamZoom = 0.1
	}
}

// SetBorder updates boundaries
func (w *World) SetBorder(l, t, r, b float64) {
	w.Border = Border{l, t, r, b}
}

// Center returns player center
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
			t += int(c.Radius * c.Radius)
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

// VisibleCells returns cells in view sorted by Y
func (w *World) VisibleCells(sw, sh int) []*Cell {
	v := make([]*Cell, 0, len(w.Cells))
	fsw, fsh := float32(sw), float32(sh)
	hw, hh := fsw/(2*w.CamZoom), fsh/(2*w.CamZoom)
	for _, c := range w.Cells {
		if c.X+c.Radius >= w.CamX-hw && c.X-c.Radius <= w.CamX+hw &&
			c.Y+c.Radius >= w.CamY-hh && c.Y-c.Radius <= w.CamY+hh {
			v = append(v, c)
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].Y < v[j].Y })
	return v
}

// Distance between two points
func Distance(x1, y1, x2, y2 float32) float32 {
	dx, dy := x2-x1, y2-y1
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}
