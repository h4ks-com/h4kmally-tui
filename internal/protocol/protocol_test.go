package protocol

import (
	"encoding/binary"
	"math"
	"testing"
)

// identity shuffle (no-op) for deterministic tests
func identityProto() *Protocol {
	p := New()
	for i := range p.shuffle {
		p.shuffle[i] = byte(i)
		p.inverse[i] = byte(i)
	}
	p.ready = true
	return p
}

func TestHandshake(t *testing.T) {
	p := New()
	hs := p.Handshake()
	if string(hs) != "SIG 0.0.2\x00" {
		t.Fatalf("want %q got %q", "SIG 0.0.2\x00", hs)
	}
}

func TestProcessHandshake(t *testing.T) {
	p := New()
	// Build a fake server handshake: version + identity shuffle table
	data := []byte("SIG 0.0.2\x00")
	for i := 0; i < 256; i++ {
		data = append(data, byte(i))
	}
	ver, err := p.ProcessHandshake(data)
	if err != nil {
		t.Fatal(err)
	}
	if ver != "SIG 0.0.2" {
		t.Fatalf("want SIG 0.0.2 got %s", ver)
	}
	if !p.IsReady() {
		t.Fatal("protocol not ready after handshake")
	}
}

func TestDecodeBorder(t *testing.T) {
	p := identityProto()
	buf := []byte{OpBorder}
	for _, v := range []float64{-14142, -14142, 14142, 14142} {
		b := [8]byte{}
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
		buf = append(buf, b[:]...)
	}
	msg, err := p.DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := msg.(BorderMsg)
	if !ok {
		t.Fatalf("want BorderMsg got %T", msg)
	}
	if b.Left != -14142 || b.Right != 14142 {
		t.Fatalf("unexpected border %+v", b)
	}
}

func TestDecodeCamera(t *testing.T) {
	p := identityProto()
	buf := []byte{OpCamera}
	for _, v := range []float32{100.5, -200.0, 0.0} {
		b := [4]byte{}
		binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
		buf = append(buf, b[:]...)
	}
	msg, err := p.DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := msg.(CameraMsg)
	if !ok {
		t.Fatalf("want CameraMsg got %T", msg)
	}
	if c.X != 100.5 || c.Y != -200.0 {
		t.Fatalf("unexpected camera %+v", c)
	}
}

func TestDecodeAddMyCell(t *testing.T) {
	p := identityProto()
	buf := []byte{OpAddMyCell, 0xAB, 0xCD, 0x00, 0x00}
	msg, err := p.DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := msg.(AddMyCellMsg)
	if !ok {
		t.Fatalf("want AddMyCellMsg got %T", msg)
	}
	if a.ID != 0x0000CDAB {
		t.Fatalf("want 0xCDAB got 0x%X", a.ID)
	}
}

func TestDecodeSpawnResult(t *testing.T) {
	p := identityProto()

	for _, accepted := range []bool{true, false} {
		v := byte(0)
		if accepted {
			v = 1
		}
		msg, err := p.DecodeMessage([]byte{OpSpawnResult, v})
		if err != nil {
			t.Fatal(err)
		}
		s, ok := msg.(SpawnResultMsg)
		if !ok {
			t.Fatalf("want SpawnResultMsg got %T", msg)
		}
		if s.Accepted != accepted {
			t.Fatalf("accepted: want %v got %v", accepted, s.Accepted)
		}
	}
}

func TestDecodeWorldUpdate(t *testing.T) {
	p := identityProto()

	// Build a world update with 1 eat event and 1 cell
	var buf []byte
	buf = append(buf, OpWorldUpdate)

	// eat count = 1
	buf = append(buf, 1, 0)
	// eat event: eater=10, eaten=20
	buf = append(buf, 10, 0, 0, 0, 20, 0, 0, 0)

	// cell id = 42
	buf = append(buf, 42, 0, 0, 0)
	// x = 100 (i16 LE)
	buf = append(buf, 100, 0)
	// y = -50 (i16 LE)
	y16 := int16(-50)
	buf = append(buf, byte(y16), byte(y16>>8))
	// size = 80 (u16 LE)
	buf = append(buf, 80, 0)
	// flags = color (0x02)
	buf = append(buf, 0x02)
	// isVirus=0, isPlayer=1, isSubscriber=0
	buf = append(buf, 0, 1, 0)
	// clan = "" (null terminator)
	buf = append(buf, 0)
	// color R=255 G=128 B=0
	buf = append(buf, 255, 128, 0)

	// sentinel: cellId=0
	buf = append(buf, 0, 0, 0, 0)

	// remove count = 0
	buf = append(buf, 0, 0)

	msg, err := p.DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := msg.(WorldUpdateMsg)
	if !ok {
		t.Fatalf("want WorldUpdateMsg got %T", msg)
	}
	if len(u.EatEvents) != 1 || u.EatEvents[0].EaterID != 10 || u.EatEvents[0].EatenID != 20 {
		t.Fatalf("unexpected eat events: %+v", u.EatEvents)
	}
	if len(u.AddCells) != 1 {
		t.Fatalf("want 1 cell got %d", len(u.AddCells))
	}
	c := u.AddCells[0]
	if c.ID != 42 || c.X != 100 || c.Y != -50 || c.Size != 80 {
		t.Fatalf("unexpected cell %+v", c)
	}
	if c.R != 255 || c.G != 128 || c.B != 0 {
		t.Fatalf("unexpected color (%d,%d,%d)", c.R, c.G, c.B)
	}
	if !c.IsPlayer {
		t.Fatal("expected isPlayer=true")
	}
}

func TestDecodeLeaderboard(t *testing.T) {
	p := identityProto()

	var buf []byte
	buf = append(buf, OpLeaderboard)
	// count = 2
	buf = append(buf, 2, 0, 0, 0)

	// entry 1: isMe=1, name="alice\0", rank=1, isSubscriber=0
	buf = append(buf, 1, 0, 0, 0)
	buf = append(buf, 'a', 'l', 'i', 'c', 'e', 0)
	buf = append(buf, 1, 0, 0, 0)
	buf = append(buf, 0, 0, 0, 0)

	// entry 2: isMe=0, name="bob\0", rank=2, isSubscriber=1
	buf = append(buf, 0, 0, 0, 0)
	buf = append(buf, 'b', 'o', 'b', 0)
	buf = append(buf, 2, 0, 0, 0)
	buf = append(buf, 1, 0, 0, 0)

	msg, err := p.DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	lb, ok := msg.(LeaderboardMsg)
	if !ok {
		t.Fatalf("want LeaderboardMsg got %T", msg)
	}
	if len(lb.Entries) != 2 {
		t.Fatalf("want 2 entries got %d", len(lb.Entries))
	}
	if lb.Entries[0].Name != "alice" || !lb.Entries[0].IsMe || lb.Entries[0].Rank != 1 {
		t.Fatalf("entry 0: %+v", lb.Entries[0])
	}
	if lb.Entries[1].Name != "bob" || lb.Entries[1].IsMe || !lb.Entries[1].IsSubscriber {
		t.Fatalf("entry 1: %+v", lb.Entries[1])
	}
}

func TestEncodeMouseMove(t *testing.T) {
	p := identityProto()
	msg := p.EncodeMouseMove(1000, -500)
	if len(msg) != 9 {
		t.Fatalf("want 9 bytes got %d", len(msg))
	}
	if msg[0] != OpMouseMove {
		t.Fatalf("wrong opcode %d", msg[0])
	}
	x := int32(binary.LittleEndian.Uint32(msg[1:5]))
	y := int32(binary.LittleEndian.Uint32(msg[5:9]))
	if x != 1000 || y != -500 {
		t.Fatalf("want (1000,-500) got (%d,%d)", x, y)
	}
}

func TestEncodeSpawn(t *testing.T) {
	p := identityProto()
	msg := p.EncodeSpawn(SpawnPayload{Name: "test"})
	if msg[0] != OpSpawn {
		t.Fatalf("wrong opcode %d", msg[0])
	}
	if msg[len(msg)-1] != 0 {
		t.Fatal("missing null terminator")
	}
}

func TestClearMessages(t *testing.T) {
	p := identityProto()
	for _, tc := range []struct {
		op   byte
		want interface{}
	}{
		{OpClearAll, ClearAllMsg{}},
		{OpClearMine, ClearMineMsg{}},
		{OpPingReply, PingReplyMsg{}},
	} {
		msg, err := p.DecodeMessage([]byte{tc.op})
		if err != nil {
			t.Fatalf("op %d: %v", tc.op, err)
		}
		if msg != tc.want {
			t.Fatalf("op %d: want %T got %T", tc.op, tc.want, msg)
		}
	}
}
