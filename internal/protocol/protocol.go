// Package protocol implements the SIG 0.0.1 binary WebSocket protocol
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

const ProtocolVersion = "SIG 0.0.1"

// Client opcodes
const (
	OpSpawn     = 0
	OpMouseMove = 1
	OpSplit     = 2
	OpEject     = 3
	OpCaptcha   = 220
)

// Server opcodes
const (
	OpWorldUpdate  = 16
	OpCamera       = 17
	OpLeaderboard  = 49
	OpBorder       = 64
	OpAddMyCell    = 32
	OpRemoveMyCell = 33
	OpSpawnResult  = 221
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
	Skin          string `json:"skin"`
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

// EncodeMouseMove creates move message
func (p *Protocol) EncodeMouseMove(x, y float32) []byte {
	msg := make([]byte, 9)
	msg[0] = p.enc(OpMouseMove)
	binary.LittleEndian.PutUint32(msg[1:5], math.Float32bits(x))
	binary.LittleEndian.PutUint32(msg[5:9], math.Float32bits(y))
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

// Message types from server
type (
	BorderMsg       struct{ Left, Top, Right, Bottom float64 }
	CameraMsg       struct{ X, Y, Zoom float32 }
	Cell            struct{ ID uint32; X, Y, Radius float32; Color uint8; Name, Skin string }
	WorldUpdateMsg  struct{ EatenCount uint16; AddCells []Cell; RemoveIDs []uint32 }
	SpawnResultMsg  struct{ Accepted bool }
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
		return p.decodeAddMyCell(payload)
	case OpRemoveMyCell:
		return p.decodeRemoveMyCell(payload)
	case OpSpawnResult:
		return p.decodeSpawnResult(payload)
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

func (p *Protocol) decodeAddMyCell(d []byte) (uint32, error) {
	if len(d) < 4 {
		return 0, fmt.Errorf("too short")
	}
	return binary.LittleEndian.Uint32(d[0:4]), nil
}

func (p *Protocol) decodeRemoveMyCell(d []byte) (uint32, error) {
	if len(d) < 4 {
		return 0, fmt.Errorf("too short")
	}
	return binary.LittleEndian.Uint32(d[0:4]), nil
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
	offset := 0
	msg.EatenCount = binary.LittleEndian.Uint16(d[offset : offset+2])
	offset += 2 + int(msg.EatenCount)*8

	for offset < len(d) {
		cell, n, err := p.decodeCell(d[offset:])
		if err != nil {
			break
		}
		msg.AddCells = append(msg.AddCells, cell)
		offset += n
	}
	return msg, nil
}

func (p *Protocol) decodeCell(d []byte) (Cell, int, error) {
	if len(d) < 15 {
		return Cell{}, 0, fmt.Errorf("too short")
	}
	c := Cell{}
	off := 0
	c.ID = binary.LittleEndian.Uint32(d[off : off+4])
	off += 4
	c.X = math.Float32frombits(binary.LittleEndian.Uint32(d[off : off+4]))
	off += 4
	c.Y = math.Float32frombits(binary.LittleEndian.Uint32(d[off : off+4]))
	off += 4
	c.Radius = math.Float32frombits(binary.LittleEndian.Uint32(d[off : off+4]))
	off += 4
	c.Color = d[off]
	off++
	flags := d[off]
	off++

	if flags&0x01 != 0 {
		end := off
		for end < len(d) && d[end] != 0 {
			end++
		}
		c.Name = string(d[off:end])
		off = end + 1
	}
	if flags&0x02 != 0 {
		end := off
		for end < len(d) && d[end] != 0 {
			end++
		}
		c.Skin = string(d[off:end])
		off = end + 1
	}
	return c, off, nil
}

// IsReady returns true if handshake complete
func (p *Protocol) IsReady() bool { return p.ready }
