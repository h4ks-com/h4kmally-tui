// Package client handles WebSocket connection
package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gorilla/websocket"
	"tui-agar/internal/protocol"
)

// Message types
type (
	ConnectedMsg     struct{}
	HandshakeDoneMsg struct{ Version string }
	ServerMsg        struct{ Msg interface{} }
	DisconnectedMsg  struct{ Err error }
)

// Client manages WebSocket
type Client struct {
	conn  *websocket.Conn
	proto *protocol.Protocol
	url   string
	mu    sync.Mutex
	msgs  chan interface{}
	done  chan struct{}
}

// New creates a client
func New(url string) *Client {
	return &Client{
		proto: protocol.New(),
		url:   url,
		msgs:  make(chan interface{}, 100),
		done:  make(chan struct{}),
	}
}

// Connect starts connection
func (c *Client) Connect() tea.Cmd {
	return func() tea.Msg {
		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
				NextProtos:         []string{"http/1.1"},
			},
		}

		headers := http.Header{
			"Origin": []string{"https://one.sigmally.com"},
		}
		conn, resp, err := dialer.Dial(c.url, headers)
		if err != nil {
			if resp != nil {
				return DisconnectedMsg{Err: fmt.Errorf("dial failed (HTTP %d): %w", resp.StatusCode, err)}
			}
			return DisconnectedMsg{Err: fmt.Errorf("dial failed: %w", err)}
		}

		c.conn = conn
		if err := conn.WriteMessage(websocket.BinaryMessage, c.proto.Handshake()); err != nil {
			conn.Close()
			return DisconnectedMsg{Err: fmt.Errorf("handshake send failed: %w", err)}
		}
		go c.readLoop()
		return ConnectedMsg{}
	}
}

func (c *Client) readLoop() {
	defer close(c.msgs)
	for {
		select {
		case <-c.done:
			return
		default:
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				c.msgs <- DisconnectedMsg{Err: err}
				return
			}

			if !c.proto.IsReady() {
				ver, err := c.proto.ProcessHandshake(data)
				if err != nil {
					c.msgs <- DisconnectedMsg{Err: err}
					return
				}
				c.msgs <- HandshakeDoneMsg{Version: ver}
				continue
			}

			msg, err := c.proto.DecodeMessage(data)
			if err != nil {
				continue // non-fatal; server may send unknown opcodes
			}
			if msg == nil {
				continue // explicitly ignored opcode (e.g. team leaderboard)
			}
			c.msgs <- ServerMsg{Msg: msg}
		}
	}
}

// Read waits for next message
func (c *Client) Read() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-c.msgs
		if !ok {
			return DisconnectedMsg{}
		}
		return msg
	}
}

// Send writes to websocket
func (c *Client) Send(data []byte) tea.Cmd {
	return func() tea.Msg {
		if c.conn == nil {
			return nil
		}
		c.mu.Lock()
		err := c.conn.WriteMessage(websocket.BinaryMessage, data)
		c.mu.Unlock()
		if err != nil {
			return DisconnectedMsg{Err: err}
		}
		return nil
	}
}

// Spawn sends spawn request
func (c *Client) Spawn(name string) tea.Cmd {
	return c.Send(c.proto.EncodeSpawn(protocol.SpawnPayload{Name: name}))
}

// Captcha sends captcha
func (c *Client) Captcha(token string) tea.Cmd {
	return c.Send(c.proto.EncodeCaptcha(token))
}

// Move sends mouse position (world coordinates as i32)
func (c *Client) Move(x, y int32) tea.Cmd {
	return c.Send(c.proto.EncodeMouseMove(x, y))
}

// Ping sends a ping
func (c *Client) Ping() tea.Cmd { return c.Send(c.proto.EncodePing()) }

// Split sends split
func (c *Client) Split() tea.Cmd { return c.Send(c.proto.EncodeSplit()) }

// Eject sends eject
func (c *Client) Eject() tea.Cmd { return c.Send(c.proto.EncodeEject()) }

// Chat sends a chat message
func (c *Client) Chat(text string) tea.Cmd { return c.Send(c.proto.EncodeChat(text)) }

// Close closes connection
func (c *Client) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()
}
