package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"tui-agar/internal/tui"
)

func main() {
	server := flag.String("server", "ws://localhost:3001/ws/", "WebSocket server URL")
	name := flag.String("name", "Player", "Player name")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	if *verbose {
		fmt.Fprintf(os.Stderr, "Connecting to %s as %s...\n", *server, *name)
	}

	m := tui.New(*server, *name)
	
	// Run with options for better terminal handling
	p := tea.NewProgram(m,
		tea.WithWindowSize(80, 24), // Default size if not detected
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		
		// Give time to see the error
		time.Sleep(2 * time.Second)
		os.Exit(1)
	}
}
