// Package protocol implements the SIG 0.0.2 binary WebSocket protocol
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

const ProtocolVersion = "SIG 0.0.2"

// Client opcodes
const (
	OpSpawn       = 0
	OpMouseMove   = 16
	OpSplit       = 17
	OpEject       = 21
	OpCaptcha     = 220
	OpPing        = 254
)

// Server opcodes
const (
	OpWorldUpdate   = 16
	OpCamera        = 17
	OpClearAll      = 18
	OpClearMine     = 20
	OpAddMyCell     = 32
	OpAddMultiCell  = 33
	OpLeaderboardT  = 48
	OpLeaderboard   = 49 // FFA leaderboard
	OpBorder        = 64
	OpSpawnResult   = 221
	OpPingReply     = 254
)

// Protocol handles encoding/decoding with opcode shuffling
type Protocol struct {
	shuffle [256]byte
	inverse [256]byte
	ready   bool
}

// New creates a protocol instance
func New() *Protocol { return &Protocol{} }

// Handshake returns handshake bytes
func (p *Protocol) Handshake() []byte {
	return []byte(ProtocolVersion + "\x00")
}

// ProcessHandshake parses server response
func (p *Protocol) ProcessHandshake(data []byte) (string, error) {
	if len(data) < 10 {
		return "", fmt.Errorf("too short")
	}
	end := 0
	for i, b := range data {
		if b == 0 {
			end = i
			break
		}
	}
	version := string(data[:end])
	start := end + 1
	if len(data) < start+256 {
		return "", fmt.Errorf("missing shuffle table")
	}
	copy(p.shuffle[:], data[start:start+256])
	for i, b := range p.shuffle {
		p.inverse[b] = byte(i)
	}
	p.ready = true
	return version, nil
}

func (p *Protocol) enc(op byte) byte {
	if !p.ready {
		return op
	}
	return p.shuffle[op]
}

func (p *Protocol) dec(wire byte) byte {
	if !p.ready {
		return wire
	}
	return p.inverse[wire]
}

// SpawnPayload for spawn message
type SpawnPayload struct {
	Name          string `json:"name"`
	Skin          string `json:"skin,omitempty"`
	Effect        string `json:"effect,omitempty"`
	ShowClanmates bool   `json:"showClanmates"`
}

// EncodeSpawn creates spawn message
func (p *Protocol) EncodeSpawn(payload SpawnPayload) []byte {
	j, _ := json.Marshal(payload)
	msg := make([]byte, 1+len(j)+1)
	msg[0] = p.enc(OpSpawn)
	copy(msg[1:], j)
	msg[len(msg)-1] = 0
	return msg
}

// EncodeMouseMove creates move message with i32 world coordinates
func (p *Protocol) EncodeMouseMove(x, y int32) []byte {
	msg := make([]byte, 9)
	msg[0] = p.enc(OpMouseMove)
	binary.LittleEndian.PutUint32(msg[1:5], uint32(x))
	binary.LittleEndian.PutUint32(msg[5:9], uint32(y))
	return msg
}

// EncodeSplit creates split message
func (p *Protocol) EncodeSplit() []byte { return []byte{p.enc(OpSplit)} }

// EncodeEject creates eject message
func (p *Protocol) EncodeEject() []byte { return []byte{p.enc(OpEject)} }

// EncodeCaptcha creates captcha message
func (p *Protocol) EncodeCaptcha(token string) []byte {
	j, _ := json.Marshal(map[string]string{"token": token})
	msg := make([]byte, 1+len(j)+1)
	msg[0] = p.enc(OpCaptcha)
	copy(msg[1:], j)
	msg[len(msg)-1] = 0
	return msg
}

// EncodePing creates a ping message
func (p *Protocol) EncodePing() []byte { return []byte{p.enc(OpPing)} }

// Message types from server
type (
	BorderMsg struct{ Left, Top, Right, Bottom float64 }
	CameraMsg struct{ X, Y, Zoom float32 }

	// Cell flags
	EatEvent struct{ EaterID, EatenID uint32 }
	Cell     struct {
		ID           uint32
		X, Y         int16
		Size         uint16
		IsVirus      bool
		IsPlayer     bool
		IsSubscriber bool
		Clan         string
		R, G, B      uint8
		Skin         string
		Name         string
		Effect       string
	}
	WorldUpdateMsg struct {
		EatEvents []EatEvent
		AddCells  []Cell
		RemoveIDs []uint32
	}

	SpawnResultMsg  struct{ Accepted bool }
	AddMyCellMsg    struct{ ID uint32 }
	AddMultiCellMsg struct{ ID uint32 }
	ClearAllMsg     struct{}
	ClearMineMsg    struct{}
	PingReplyMsg    struct{}

	LeaderboardEntry struct {
		Name         string
		Rank         uint32
		IsMe         bool
		IsSubscriber bool
	}
	LeaderboardMsg struct {
		Entries []LeaderboardEntry
	}
)

// Cell flag bits
const (
	FlagColor  = 0x02
	FlagSkin   = 0x04
	FlagName   = 0x08
	FlagEffect = 0x10
)

// DecodeMessage decodes a server message
func (p *Protocol) DecodeMessage(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty")
	}
	op := p.dec(data[0])
	payload := data[1:]

	switch op {
	case OpBorder:
		return p.decodeBorder(payload)
	case OpCamera:
		return p.decodeCamera(payload)
	case OpWorldUpdate:
		return p.decodeWorldUpdate(payload)
	case OpAddMyCell:
		if len(payload) < 4 {
			return nil, fmt.Errorf("AddMyCell too short")
		}
		return AddMyCellMsg{ID: binary.LittleEndian.Uint32(payload[0:4])}, nil
	case OpAddMultiCell:
		if len(payload) < 4 {
			return nil, fmt.Errorf("AddMultiCell too short")
		}
		return AddMultiCellMsg{ID: binary.LittleEndian.Uint32(payload[0:4])}, nil
	case OpSpawnResult:
		return p.decodeSpawnResult(payload)
	case OpClearAll:
		return ClearAllMsg{}, nil
	case OpClearMine:
		return ClearMineMsg{}, nil
	case OpPingReply:
		return PingReplyMsg{}, nil
	case OpLeaderboard:
		return p.decodeLeaderboard(payload)
	case OpLeaderboardT:
		return nil, nil // team leaderboard — not used, ignore cleanly
	}
	return nil, fmt.Errorf("unknown opcode %d", op)
}

func (p *Protocol) decodeBorder(d []byte) (BorderMsg, error) {
	if len(d) < 32 {
		return BorderMsg{}, fmt.Errorf("too short")
	}
	return BorderMsg{
		Left:   math.Float64frombits(binary.LittleEndian.Uint64(d[0:8])),
		Top:    math.Float64frombits(binary.LittleEndian.Uint64(d[8:16])),
		Right:  math.Float64frombits(binary.LittleEndian.Uint64(d[16:24])),
		Bottom: math.Float64frombits(binary.LittleEndian.Uint64(d[24:32])),
	}, nil
}

func (p *Protocol) decodeCamera(d []byte) (CameraMsg, error) {
	if len(d) < 12 {
		return CameraMsg{}, fmt.Errorf("too short")
	}
	return CameraMsg{
		X:    math.Float32frombits(binary.LittleEndian.Uint32(d[0:4])),
		Y:    math.Float32frombits(binary.LittleEndian.Uint32(d[4:8])),
		Zoom: math.Float32frombits(binary.LittleEndian.Uint32(d[8:12])),
	}, nil
}

func (p *Protocol) decodeSpawnResult(d []byte) (SpawnResultMsg, error) {
	if len(d) < 1 {
		return SpawnResultMsg{}, fmt.Errorf("too short")
	}
	return SpawnResultMsg{Accepted: d[0] != 0}, nil
}

func (p *Protocol) decodeWorldUpdate(d []byte) (WorldUpdateMsg, error) {
	msg := WorldUpdateMsg{}
	if len(d) < 2 {
		return msg, nil
	}
	off := 0

	// Section 1: eat events
	eatCount := int(binary.LittleEndian.Uint16(d[off : off+2]))
	off += 2
	if off+eatCount*8 > len(d) {
		return msg, fmt.Errorf("eat events overflow")
	}
	for i := 0; i < eatCount; i++ {
		eater := binary.LittleEndian.Uint32(d[off : off+4])
		eaten := binary.LittleEndian.Uint32(d[off+4 : off+8])
		msg.EatEvents = append(msg.EatEvents, EatEvent{EaterID: eater, EatenID: eaten})
		off += 8
	}

	// Section 2: cell updates until sentinel (cellId == 0)
	for off+4 <= len(d) {
		cellID := binary.LittleEndian.Uint32(d[off : off+4])
		off += 4
		if cellID == 0 {
			break
		}
		cell, n, err := p.decodeCell(d[off:], cellID)
		if err != nil {
			break
		}
		msg.AddCells = append(msg.AddCells, cell)
		off += n
	}

	// Section 3: removals
	if off+2 <= len(d) {
		removeCount := int(binary.LittleEndian.Uint16(d[off : off+2]))
		off += 2
		for i := 0; i < removeCount && off+4 <= len(d); i++ {
			msg.RemoveIDs = append(msg.RemoveIDs, binary.LittleEndian.Uint32(d[off:off+4]))
			off += 4
		}
	}

	return msg, nil
}

func (p *Protocol) decodeCell(d []byte, id uint32) (Cell, int, error) {
	// Need at least: i16 x + i16 y + u16 size + u8 flags + u8 isVirus + u8 isPlayer + u8 isSubscriber + clan(\0) = 10 bytes min
	if len(d) < 10 {
		return Cell{}, 0, fmt.Errorf("cell too short: %d", len(d))
	}
	c := Cell{ID: id}
	off := 0

	c.X = int16(binary.LittleEndian.Uint16(d[off : off+2]))
	off += 2
	c.Y = int16(binary.LittleEndian.Uint16(d[off : off+2]))
	off += 2
	c.Size = binary.LittleEndian.Uint16(d[off : off+2])
	off += 2
	flags := d[off]
	off++
	c.IsVirus = d[off] != 0
	off++
	c.IsPlayer = d[off] != 0
	off++
	c.IsSubscriber = d[off] != 0
	off++

	// clan string (null-terminated)
	clan, n, err := readString(d[off:])
	if err != nil {
		return Cell{}, 0, fmt.Errorf("clan: %w", err)
	}
	c.Clan = clan
	off += n

	if flags&FlagColor != 0 {
		if off+3 > len(d) {
			return Cell{}, 0, fmt.Errorf("color overflow")
		}
		c.R, c.G, c.B = d[off], d[off+1], d[off+2]
		off += 3
	}
	if flags&FlagSkin != 0 {
		s, n, err := readString(d[off:])
		if err != nil {
			return Cell{}, 0, fmt.Errorf("skin: %w", err)
		}
		c.Skin = s
		off += n
	}
	if flags&FlagName != 0 {
		s, n, err := readString(d[off:])
		if err != nil {
			return Cell{}, 0, fmt.Errorf("name: %w", err)
		}
		c.Name = s
		off += n
	}
	if flags&FlagEffect != 0 {
		s, n, err := readString(d[off:])
		if err != nil {
			return Cell{}, 0, fmt.Errorf("effect: %w", err)
		}
		c.Effect = s
		off += n
	}

	return c, off, nil
}

func (p *Protocol) decodeLeaderboard(d []byte) (LeaderboardMsg, error) {
	msg := LeaderboardMsg{}
	if len(d) < 4 {
		return msg, nil
	}
	count := int(binary.LittleEndian.Uint32(d[0:4]))
	off := 4
	for i := 0; i < count && off < len(d); i++ {
		if off+4 > len(d) {
			break
		}
		isMe := binary.LittleEndian.Uint32(d[off:off+4]) != 0
		off += 4
		name, n, err := readString(d[off:])
		if err != nil {
			break
		}
		off += n
		if off+8 > len(d) {
			break
		}
		rank := binary.LittleEndian.Uint32(d[off : off+4])
		off += 4
		isSub := binary.LittleEndian.Uint32(d[off:off+4]) != 0
		off += 4
		msg.Entries = append(msg.Entries, LeaderboardEntry{
			Name:         name,
			Rank:         rank,
			IsMe:         isMe,
			IsSubscriber: isSub,
		})
	}
	return msg, nil
}

// readString reads a null-terminated UTF-8 string, returns (string, bytes_consumed, error).
// Caps search at 256 bytes — player names, clans, and skin names are never longer.
func readString(d []byte) (string, int, error) {
	if len(d) > 256 {
		d = d[:256]
	}
	for i, b := range d {
		if b == 0 {
			return string(d[:i]), i + 1, nil
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}

// IsReady returns true if handshake complete
func (p *Protocol) IsReady() bool { return p.ready }
