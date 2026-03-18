// Package client handles WebSocket connection
package client

import (
	"crypto/tls"
	"fmt"
	"os"
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
	msgs  chan []byte
	done  chan struct{}
}

// New creates a client
func New(url string) *Client {
	return &Client{
		proto: protocol.New(),
		url:   url,
		msgs:  make(chan []byte, 100),
		done:  make(chan struct{}),
	}
}

// URL returns the server URL
func (c *Client) URL() string {
	return c.url
}

// Connect starts connection
func (c *Client) Connect() tea.Cmd {
	return func() tea.Msg {
		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		}
		
		fmt.Fprintf(os.Stderr, "[DEBUG] Dialing %s...\n", c.url)
		conn, _, err := dialer.Dial(c.url, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Dial failed: %v\n", err)
			return DisconnectedMsg{Err: fmt.Errorf("dial failed: %w", err)}
		}
		fmt.Fprintf(os.Stderr, "[DEBUG] Connected! Sending handshake...\n")
		
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
				fmt.Fprintf(os.Stderr, "[DEBUG] Read error: %v\n", err)
				c.msgs <- nil
				return
			}
			c.msgs <- data
		}
	}
}

// Read waits for next message
func (c *Client) Read() tea.Cmd {
	return func() tea.Msg {
		data, ok := <-c.msgs
		if !ok || data == nil {
			return DisconnectedMsg{Err: fmt.Errorf("connection closed")}
		}
		if !c.proto.IsReady() {
			ver, err := c.proto.ProcessHandshake(data)
			if err != nil {
				return DisconnectedMsg{Err: fmt.Errorf("handshake failed: %w", err)}
			}
			fmt.Fprintf(os.Stderr, "[DEBUG] Handshake complete: %s\n", ver)
			return HandshakeDoneMsg{Version: ver}
		}
		msg, err := c.proto.DecodeMessage(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Decode error: %v\n", err)
			return nil
		}
		return ServerMsg{Msg: msg}
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

// Move sends mouse position
func (c *Client) Move(x, y float32) tea.Cmd {
	return c.Send(c.proto.EncodeMouseMove(x, y))
}

// Split sends split
func (c *Client) Split() tea.Cmd { return c.Send(c.proto.EncodeSplit()) }

// Eject sends eject
func (c *Client) Eject() tea.Cmd { return c.Send(c.proto.EncodeEject()) }

// Close closes connection
func (c *Client) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
