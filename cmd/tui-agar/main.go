package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"tui-agar/internal/tui"
)

func main() {
	server := flag.String("server", "ws://localhost:3001/ws/", "WebSocket server URL")
	name := flag.String("name", "Player", "Player name")
	flag.Parse()

	m := tui.New(*server, *name)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
