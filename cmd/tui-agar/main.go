package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"tui-agar/internal/tui"
)

// syncWriter wraps terminal writes in Mode 2026 synchronized output sequences,
// telling the terminal to buffer all updates and display the frame atomically
// only when the flush sequence arrives. This prevents partial-frame rendering
// artifacts over SSH, where TCP packet boundaries can cause the terminal to
// display mid-update state. Terminals that do not support Mode 2026 silently
// ignore the sequences.
//
// Writes containing alt-screen sequences (\x1b[?1049) are passed through
// unmodified. Bubble Tea's cursed renderer batches the alt-screen enter with
// the first frame in a single Write() call; wrapping that in Mode 2026 causes
// terminals such as tmux to buffer the alt-screen switch, producing a black
// screen. Subsequent frame writes (pure differential updates) contain no
// alt-screen sequences and are wrapped normally.
type syncWriter struct {
	w io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("\x1b[?1049")) {
		return s.w.Write(p)
	}
	const open = "\x1b[?2026h"
	const close = "\x1b[?2026l"
	buf := make([]byte, len(open)+len(p)+len(close))
	n := copy(buf, open)
	n += copy(buf[n:], p)
	copy(buf[n:], close)
	_, err := s.w.Write(buf)
	return len(p), err
}

// Fd, Read, Close forward to the underlying writer when it supports those
// interfaces. This satisfies charm.land/bubbletea/v2's term.File interface so
// that Bubble Tea detects the wrapped output as a TTY and initialises the
// renderer. Without Fd(), p.ttyOutput stays nil and no renderer is created.
func (s *syncWriter) Fd() uintptr {
	if f, ok := s.w.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return ^uintptr(0)
}

func (s *syncWriter) Read(p []byte) (int, error) {
	if r, ok := s.w.(io.Reader); ok {
		return r.Read(p)
	}
	return 0, io.EOF
}

func (s *syncWriter) Close() error {
	if c, ok := s.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func main() {
	server := flag.String("server", "ws://localhost:3001/ws/", "WebSocket server URL")
	name := flag.String("name", "Player", "Player name")
	test := flag.Bool("test", false, "Run in test mode (no TTY)")
	sync := flag.Bool("sync", false, "Wrap output in Mode 2026 synchronized updates (recommended over SSH)")
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
		// Normal mode: use real terminal.
		// syncWriter wraps output in Mode 2026 synchronized updates, which
		// prevents partial-frame artifacts over SSH. Enable with -sync when
		// running behind an SSH proxy (e.g. the sshland Dockerfile).
		var out io.Writer = os.Stdout
		if *sync {
			out = &syncWriter{w: os.Stdout}
		}
		p = tea.NewProgram(m,
			tea.WithInput(os.Stdin),
			tea.WithOutput(out),
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
