package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"tui-agar/internal/tui"
)

func main() {
	server := flag.String("server", "ws://localhost:3001/ws/", "WebSocket server URL")
	name := flag.String("name", "Player", "Player name")
	test := flag.Bool("test", false, "Run in test mode (no TTY)")
	flag.Parse()

	// Log to file for debugging
	logFile, err := os.Create("/tmp/agar-debug.log")
	if err != nil {
		log.Fatalf("Cannot create log: %v", err)
	}
	log.SetOutput(logFile)

	log.Printf("Starting client: server=%s name=%s test=%v", *server, *name, *test)

	m := tui.New(*server, *name)

	var p *tea.Program
	if *test {
		// Test mode: run without TTY
		p = tea.NewProgram(m,
			tea.WithWindowSize(80, 24),
			tea.WithInput(bytes.NewReader(nil)),
			tea.WithOutput(os.Stderr),
		)
	} else {
		// Normal mode: use real terminal
		p = tea.NewProgram(m,
			tea.WithInput(os.Stdin),
			tea.WithOutput(os.Stdout),
		)
	}

	log.Printf("Running program...")

	if *test {
		done := make(chan error, 1)
		go func() {
			_, err := p.Run()
			done <- err
		}()

		select {
		case err := <-done:
			if err != nil {
				log.Printf("Program error: %v", err)
				os.Exit(1)
			}
			log.Printf("Program exited normally")
		case <-time.After(10 * time.Second):
			log.Printf("Timeout - sending quit")
			p.Quit()
			<-done
			log.Printf("Quit complete")
		}
	} else {
		if _, err := p.Run(); err != nil {
			log.Printf("Program error: %v", err)
			os.Exit(1)
		}
		log.Printf("Program exited normally")
	}
}
