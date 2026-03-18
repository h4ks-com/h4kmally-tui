package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"tui-agar/internal/protocol"
)

func main() {
	server := flag.String("server", "ws://localhost:3001/ws/", "WebSocket server URL")
	name := flag.String("name", "TestBot", "Player name")
	flag.Parse()

	log.Printf("Connecting to %s...", *server)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(*server, nil)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()
	log.Printf("Connected!")

	// Send handshake
	proto := protocol.New()
	handshake := proto.Handshake()
	log.Printf("Sending handshake: %q", handshake[:len(handshake)-1])
	if err := conn.WriteMessage(websocket.BinaryMessage, handshake); err != nil {
		log.Fatalf("Handshake send failed: %v", err)
	}

	// Read handshake response
	_, data, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("Handshake read failed: %v", err)
	}
	log.Printf("Received handshake response: %d bytes", len(data))

	version, err := proto.ProcessHandshake(data)
	if err != nil {
		log.Fatalf("ProcessHandshake failed: %v", err)
	}
	log.Printf("Handshake complete! Protocol version: %s", version)

	// Send captcha skip
	captcha := proto.EncodeCaptcha("skip")
	if err := conn.WriteMessage(websocket.BinaryMessage, captcha); err != nil {
		log.Fatalf("Captcha send failed: %v", err)
	}
	log.Printf("Sent captcha skip")

	// Send spawn
	spawn := proto.EncodeSpawn(protocol.SpawnPayload{Name: *name})
	if err := conn.WriteMessage(websocket.BinaryMessage, spawn); err != nil {
		log.Fatalf("Spawn send failed: %v", err)
	}
	log.Printf("Sent spawn request as '%s'", *name)

	// Read messages for 10 seconds
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	msgCount := 0
	for {
		select {
		case <-ticker.C:
			log.Printf("Timeout - received %d messages", msgCount)
			return
		case <-interrupt:
			log.Printf("Interrupted - received %d messages", msgCount)
			return
		default:
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Printf("Server closed connection")
					return
				}
				continue
			}
			
			msg, err := proto.DecodeMessage(data)
			if err != nil {
				log.Printf("Decode error: %v", err)
				continue
			}
			
			msgCount++
			switch v := msg.(type) {
			case protocol.BorderMsg:
				log.Printf("[%d] BORDER: L=%.0f T=%.0f R=%.0f B=%.0f", msgCount, v.Left, v.Top, v.Right, v.Bottom)
			case protocol.CameraMsg:
				log.Printf("[%d] CAMERA: X=%.1f Y=%.1f Zoom=%.3f", msgCount, v.X, v.Y, v.Zoom)
			case protocol.WorldUpdateMsg:
				log.Printf("[%d] WORLD_UPDATE: %d cells, eaten=%d", msgCount, len(v.AddCells), v.EatenCount)
			case uint32:
				log.Printf("[%d] ADD_MY_CELL: %d", msgCount, v)
			case protocol.SpawnResultMsg:
				log.Printf("[%d] SPAWN_RESULT: accepted=%v", msgCount, v.Accepted)
			default:
				log.Printf("[%d] Unknown message type: %T", msgCount, msg)
			}
		}
	}
}
