package game

import (
	"math"
	"testing"
)

func TestViewZoomDefault(t *testing.T) {
	w := NewWorld()
	// no cells → default zoom
	z := w.ViewZoom()
	if z != 0.05 {
		t.Fatalf("want 0.05 got %v", z)
	}
}

func TestViewZoomClamped(t *testing.T) {
	w := NewWorld()

	// huge cell → clamped to min: 0.7/sqrt(r) < 0.008 when r > (0.7/0.008)^2 ≈ 7656
	w.Cells[1] = &Cell{ID: 1, Radius: 10000}
	w.MyCells = []uint32{1}
	z := w.ViewZoom()
	if z != 0.008 {
		t.Fatalf("want 0.008 got %v", z)
	}

	// tiny cell → clamped to max
	w.Cells[1].Radius = 1
	z = w.ViewZoom()
	if z != 0.15 {
		t.Fatalf("want 0.15 got %v", z)
	}
}

func TestViewZoomFormula(t *testing.T) {
	w := NewWorld()
	w.Cells[1] = &Cell{ID: 1, Radius: 100}
	w.MyCells = []uint32{1}

	z := w.ViewZoom()
	// 0.7 / sqrt(100) = 0.07
	want := float32(0.7 / math.Sqrt(100))
	if math.Abs(float64(z-want)) > 1e-5 {
		t.Fatalf("want %v got %v", want, z)
	}
}

func TestToScreenRoundtrip(t *testing.T) {
	w := NewWorld()
	w.CamX, w.CamY, w.CamZoom = 100, -50, 0.1

	sx, sy := w.ToScreen(200, 0, 800, 600)
	wx, wy := w.ToWorld(float32(sx), float32(sy), 800, 600)

	if math.Abs(float64(wx-200)) > 0.1 || math.Abs(float64(wy-0)) > 0.1 {
		t.Fatalf("roundtrip mismatch: got (%v,%v)", wx, wy)
	}
}

func TestToScreenCenter(t *testing.T) {
	w := NewWorld()
	w.CamX, w.CamY, w.CamZoom = 0, 0, 1.0

	// Camera center should map to screen center
	sx, sy := w.ToScreen(0, 0, 800, 600)
	if sx != 400 || sy != 300 {
		t.Fatalf("want (400,300) got (%d,%d)", sx, sy)
	}
}

func TestScreenRadius(t *testing.T) {
	w := NewWorld()
	w.CamZoom = 0.1
	if w.ScreenRadius(100) != 10 {
		t.Fatalf("want 10 got %d", w.ScreenRadius(100))
	}
}

func TestAddCell(t *testing.T) {
	w := NewWorld()
	w.AddCell(1, 100, -50, 80, 255, 128, 0, "alice", "")

	c := w.Cells[1]
	if c == nil {
		t.Fatal("cell not added")
	}
	if c.X != 100 || c.Y != -50 || c.Radius != 80 {
		t.Fatalf("unexpected cell coords/size: %+v", c)
	}
	if c.R != 255 || c.G != 128 || c.B != 0 {
		t.Fatalf("unexpected color: (%d,%d,%d)", c.R, c.G, c.B)
	}
	if c.Name != "alice" {
		t.Fatalf("unexpected name: %s", c.Name)
	}
}

func TestAddCellUpdatesExisting(t *testing.T) {
	w := NewWorld()
	w.AddCell(1, 0, 0, 50, 255, 0, 0, "bob", "")
	w.AddCell(1, 200, 100, 120, 0, 255, 0, "", "") // update, no new name

	c := w.Cells[1]
	if c.X != 200 || c.Y != 100 || c.Radius != 120 {
		t.Fatalf("cell not updated: %+v", c)
	}
	// name preserved when update sends empty string
	if c.Name != "bob" {
		t.Fatalf("name should be preserved, got %q", c.Name)
	}
}

func TestRemoveCell(t *testing.T) {
	w := NewWorld()
	w.AddCell(1, 0, 0, 50, 255, 255, 255, "", "")
	w.MyCells = []uint32{1}
	w.RemoveCell(1)

	if w.Cells[1] != nil {
		t.Fatal("cell still present after remove")
	}
	if len(w.MyCells) != 0 {
		t.Fatalf("MyCells not cleaned: %v", w.MyCells)
	}
}

func TestAddMyCell(t *testing.T) {
	w := NewWorld()
	w.AddMyCell(5)
	if len(w.MyCells) != 1 || w.MyCells[0] != 5 {
		t.Fatalf("MyCells: %v", w.MyCells)
	}
	if !w.Cells[5].IsMine {
		t.Fatal("IsMine not set")
	}
	// adding again should not duplicate
	w.AddMyCell(5)
	if len(w.MyCells) != 1 {
		t.Fatalf("duplicate in MyCells: %v", w.MyCells)
	}
}

func TestClearAll(t *testing.T) {
	w := NewWorld()
	w.AddCell(1, 0, 0, 50, 0, 0, 0, "", "")
	w.AddCell(2, 0, 0, 50, 0, 0, 0, "", "")
	w.MyCells = []uint32{1}
	w.ClearAll()

	if len(w.Cells) != 0 {
		t.Fatalf("cells not cleared: %d remaining", len(w.Cells))
	}
	if len(w.MyCells) != 0 {
		t.Fatalf("MyCells not cleared: %v", w.MyCells)
	}
}

func TestClearMine(t *testing.T) {
	w := NewWorld()
	w.AddCell(1, 0, 0, 50, 0, 0, 0, "", "")
	w.AddCell(2, 0, 0, 50, 0, 0, 0, "", "") // other player's cell
	w.MyCells = []uint32{1}
	w.ClearMine()

	if w.Cells[1] != nil {
		t.Fatal("my cell should be removed")
	}
	if w.Cells[2] == nil {
		t.Fatal("other cell should remain")
	}
	if len(w.MyCells) != 0 {
		t.Fatalf("MyCells not cleared: %v", w.MyCells)
	}
}

func TestIsAlive(t *testing.T) {
	w := NewWorld()
	if w.IsAlive() {
		t.Fatal("should not be alive with no cells")
	}
	w.MyCells = []uint32{1}
	if !w.IsAlive() {
		t.Fatal("should be alive with cells")
	}
}

func TestCenter(t *testing.T) {
	w := NewWorld()
	// no cells → camera position
	w.CamX, w.CamY = 10, 20
	cx, cy := w.Center()
	if cx != 10 || cy != 20 {
		t.Fatalf("want (10,20) got (%v,%v)", cx, cy)
	}

	// single cell
	w.AddCell(1, 100, 200, 50, 0, 0, 0, "", "")
	w.MyCells = []uint32{1}
	cx, cy = w.Center()
	if cx != 100 || cy != 200 {
		t.Fatalf("want (100,200) got (%v,%v)", cx, cy)
	}
}

func TestCenterMassWeighted(t *testing.T) {
	w := NewWorld()
	// Two cells of equal size → center is midpoint
	w.AddCell(1, 0, 0, 10, 0, 0, 0, "", "")
	w.AddCell(2, 100, 0, 10, 0, 0, 0, "", "")
	w.MyCells = []uint32{1, 2}
	cx, cy := w.Center()
	if math.Abs(float64(cx-50)) > 0.1 || cy != 0 {
		t.Fatalf("want (50,0) got (%v,%v)", cx, cy)
	}
}

func TestScore(t *testing.T) {
	w := NewWorld()
	// Score = sum(r*r/100) for my cells
	w.AddCell(1, 0, 0, 100, 0, 0, 0, "", "") // 100*100/100 = 100
	w.AddCell(2, 0, 0, 50, 0, 0, 0, "", "")  // 50*50/100 = 25
	w.MyCells = []uint32{1, 2}

	if s := w.Score(); s != 125 {
		t.Fatalf("want 125 got %d", s)
	}
}

func TestVisibleCells(t *testing.T) {
	w := NewWorld()
	w.CamX, w.CamY, w.CamZoom = 0, 0, 1.0

	// Cell at origin — visible in 100x100 screen
	w.AddCell(1, 0, 0, 5, 0, 0, 0, "", "")
	// Cell far away — not visible
	w.AddCell(2, 10000, 10000, 5, 0, 0, 0, "", "")

	visible := w.VisibleCells(100, 100)
	if len(visible) != 1 || visible[0].ID != 1 {
		t.Fatalf("expected only cell 1 visible, got %v", visible)
	}
}

func TestVisibleCellsSortedSmallToLarge(t *testing.T) {
	w := NewWorld()
	w.CamX, w.CamY, w.CamZoom = 0, 0, 0.1

	w.AddCell(1, 0, 0, 200, 0, 0, 0, "", "")
	w.AddCell(2, 0, 0, 50, 0, 0, 0, "", "")
	w.AddCell(3, 0, 0, 100, 0, 0, 0, "", "")

	visible := w.VisibleCells(800, 600)
	if len(visible) != 3 {
		t.Fatalf("want 3 cells got %d", len(visible))
	}
	if visible[0].Radius > visible[1].Radius || visible[1].Radius > visible[2].Radius {
		t.Fatalf("not sorted small→large: %v %v %v", visible[0].Radius, visible[1].Radius, visible[2].Radius)
	}
}

func TestDistance(t *testing.T) {
	d := Distance(0, 0, 3, 4)
	if math.Abs(float64(d-5)) > 1e-5 {
		t.Fatalf("want 5 got %v", d)
	}
	d = Distance(1, 1, 1, 1)
	if d != 0 {
		t.Fatalf("want 0 got %v", d)
	}
}

func TestSetBorder(t *testing.T) {
	w := NewWorld()
	w.SetBorder(-14142, -14142, 14142, 14142)
	if w.Border.Left != -14142 || w.Border.Right != 14142 {
		t.Fatalf("unexpected border: %+v", w.Border)
	}
}

func TestSetCamera(t *testing.T) {
	w := NewWorld()
	w.SetCamera(100, 200, 0.5)
	if w.CamX != 100 || w.CamY != 200 || w.CamZoom != 0.5 {
		t.Fatalf("unexpected camera: %+v", w)
	}
	// zoom=0 from server should not overwrite current zoom
	w.SetCamera(100, 200, 0)
	if w.CamZoom != 0.5 {
		t.Fatalf("zoom should be preserved when server sends 0, got %v", w.CamZoom)
	}
}
