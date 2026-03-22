package tui

import (
	"strings"
	"testing"
	"time"

	"tui-agar/internal/game"
)

// --- helpers -----------------------------------------------------------------

// chatModel returns a playing model already in chat mode.
// client is nil intentionally: chat-mode logic must never reach client code.
func chatModel(input string) Model {
	return Model{
		world:     game.NewWorld(),
		state:     statePlaying,
		w:         80,
		h:         24,
		chatMode:  true,
		chatInput: input,
	}
}

// gameModel returns a playing model with chat inactive.
func gameModel() Model {
	return Model{
		world: game.NewWorld(),
		state: statePlaying,
		w:     80,
		h:     24,
	}
}

// --- chat context ------------------------------------------------------------

// Every key that has a game binding must be treated as plain text in chat mode,
// not trigger its game action.
func TestChat_GameKeysAreText(t *testing.T) {
	cases := []struct {
		key       string
		wantInput string
	}{
		{"w", "w"}, // would eject in game
		{"r", "r"}, // would respawn in game
		{"q", "q"}, // would quit in game
		{"a", "a"}, // plain letter
		{"1", "1"}, // digit
		{"!", "!"}, // punctuation
	}
	for _, tc := range cases {
		m := chatModel("")
		got, cmd := m.handleChatKey(tc.key)
		if got.chatInput != tc.wantInput {
			t.Errorf("key=%q: chatInput got %q, want %q", tc.key, got.chatInput, tc.wantInput)
		}
		if !got.chatMode {
			t.Errorf("key=%q: chatMode must stay true", tc.key)
		}
		if cmd != nil {
			t.Errorf("key=%q: expected nil cmd, got non-nil", tc.key)
		}
	}
}

// Space must append a space character, never trigger Split.
func TestChat_SpaceAppendsSpace(t *testing.T) {
	m := chatModel("hello")
	got, cmd := m.handleChatKey("space")
	if got.chatInput != "hello " {
		t.Errorf("chatInput got %q, want %q", got.chatInput, "hello ")
	}
	if !got.chatMode {
		t.Error("chatMode must stay true after space")
	}
	if cmd != nil {
		t.Error("no cmd expected for space in chat mode")
	}
}

// Esc discards input and exits chat mode — no message sent.
func TestChat_EscDiscardsAndExits(t *testing.T) {
	m := chatModel("unsent message")
	got, cmd := m.handleChatKey("esc")
	if got.chatMode {
		t.Error("chatMode must be false after esc")
	}
	if got.chatInput != "" {
		t.Errorf("chatInput must be empty after esc, got %q", got.chatInput)
	}
	if cmd != nil {
		t.Error("no cmd expected for esc")
	}
}

// Enter with text clears the input, exits chat, and returns a send command.
func TestChat_EnterWithTextSendsAndExits(t *testing.T) {
	m := chatModel("hello world")
	got, _ := m.handleChatKey("enter")
	if got.chatMode {
		t.Error("chatMode must be false after enter")
	}
	if got.chatInput != "" {
		t.Errorf("chatInput must be cleared after enter, got %q", got.chatInput)
	}
}

// Enter with empty input exits chat without sending anything.
func TestChat_EnterEmptyExitsNoSend(t *testing.T) {
	m := chatModel("")
	got, cmd := m.handleChatKey("enter")
	if got.chatMode {
		t.Error("chatMode must be false after enter with empty input")
	}
	if cmd != nil {
		t.Error("no cmd expected when entering on empty input")
	}
}

// Backspace removes the last character.
func TestChat_BackspaceDeletesLast(t *testing.T) {
	m := chatModel("hello")
	got, _ := m.handleChatKey("backspace")
	if got.chatInput != "hell" {
		t.Errorf("got %q, want %q", got.chatInput, "hell")
	}
	if !got.chatMode {
		t.Error("chatMode must stay true after backspace")
	}
}

// Backspace on empty input is a no-op.
func TestChat_BackspaceOnEmptyIsNoop(t *testing.T) {
	m := chatModel("")
	got, _ := m.handleChatKey("backspace")
	if got.chatInput != "" {
		t.Errorf("chatInput must stay empty, got %q", got.chatInput)
	}
	if !got.chatMode {
		t.Error("chatMode must stay true")
	}
}

// Named non-printable keys (arrows, F-keys, etc.) are silently ignored.
func TestChat_NonPrintableKeysIgnored(t *testing.T) {
	for _, key := range []string{"up", "down", "left", "right", "f1", "tab", "ctrl+a"} {
		m := chatModel("x")
		got, cmd := m.handleChatKey(key)
		if got.chatInput != "x" {
			t.Errorf("key=%q: chatInput must be unchanged, got %q", key, got.chatInput)
		}
		if !got.chatMode {
			t.Errorf("key=%q: chatMode must stay true", key)
		}
		if cmd != nil {
			t.Errorf("key=%q: expected nil cmd", key)
		}
	}
}

// Input is capped at 100 characters.
func TestChat_InputLengthCap(t *testing.T) {
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	m := chatModel(string(long))
	got, _ := m.handleChatKey("z")
	if len(got.chatInput) != 100 {
		t.Errorf("input length must stay at 100, got %d", len(got.chatInput))
	}
	if got.chatInput[99] != 'a' {
		t.Error("overflow char must be dropped, not appended")
	}
}

// --- game context ------------------------------------------------------------

// Enter in game mode activates chat.
func TestGame_EnterActivatesChat(t *testing.T) {
	m := gameModel()
	got, _ := m.handleGameKey("enter")
	if !got.chatMode {
		t.Error("enter in game mode must activate chatMode")
	}
}

// Space in game mode does NOT activate chat.
func TestGame_SpaceDoesNotActivateChat(t *testing.T) {
	m := gameModel()
	got, _ := m.handleGameKey("space")
	if got.chatMode {
		t.Error("space in game mode must not activate chatMode")
	}
}

// W in game mode does NOT activate chat.
func TestGame_WDoesNotActivateChat(t *testing.T) {
	m := gameModel()
	got, _ := m.handleGameKey("w")
	if got.chatMode {
		t.Error("w in game mode must not activate chatMode")
	}
}

// Enter in non-playing states must not activate chat.
func TestGame_EnterOnlyActivatesChatWhenPlaying(t *testing.T) {
	for _, s := range []state{stateDead, stateConnecting, stateHandshake, stateError} {
		m := gameModel()
		m.state = s
		got, _ := m.handleGameKey("enter")
		if got.chatMode {
			t.Errorf("state=%v: enter must not activate chat when not playing", s)
		}
	}
}

// --- wordWrap -----------------------------------------------------------------

func TestWordWrap_Short(t *testing.T) {
	got := wordWrap("hello", 10)
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("want [hello], got %v", got)
	}
}

func TestWordWrap_ExactWidth(t *testing.T) {
	got := wordWrap("hello", 5)
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("want [hello], got %v", got)
	}
}

func TestWordWrap_OneBreak(t *testing.T) {
	got := wordWrap("hello world", 7)
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("want [hello world], got %v", got)
	}
}

func TestWordWrap_MultipleBreaks(t *testing.T) {
	text := "one two three four five"
	got := wordWrap(text, 9)
	// Each line must be <= 9 chars; joining with spaces must round-trip.
	for _, line := range got {
		if len(line) > 9 {
			t.Errorf("line %q exceeds maxWidth 9", line)
		}
	}
	if strings.Join(got, " ") != text {
		t.Errorf("round-trip failed: %q", strings.Join(got, " "))
	}
}

func TestWordWrap_NoSpaces(t *testing.T) {
	// A single token longer than maxWidth must be hard-split.
	got := wordWrap("abcdefgh", 3)
	for _, line := range got {
		if len(line) > 3 {
			t.Errorf("hard-split line %q exceeds 3", line)
		}
	}
	if strings.Join(got, "") != "abcdefgh" {
		t.Errorf("hard-split lost content: %v", got)
	}
}

func TestWordWrap_Empty(t *testing.T) {
	got := wordWrap("", 10)
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("want [\"\"], got %v", got)
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	// maxWidth <= 0 returns the text unchanged in a single element.
	got := wordWrap("hello world", 0)
	if len(got) != 1 || got[0] != "hello world" {
		t.Fatalf("want [hello world], got %v", got)
	}
}

// --- stampChat pixel-buffer tests --------------------------------------------

func makeTestBuf(w, h int) [][]pixel {
	buf := make([][]pixel, h)
	for i := range buf {
		buf[i] = make([]pixel, w)
		for j := range buf[i] {
			buf[i][j] = pixel{ch: ' '}
		}
	}
	return buf
}

func TestStampChat_ShortMessageSingleLine(t *testing.T) {
	m := Model{
		world: game.NewWorld(),
		w:     80, h: 24,
		chatMessages: []ChatMessage{
			{Name: "alice", R: 200, G: 100, B: 50, Text: "hi", At: time.Now()},
		},
	}
	buf := makeTestBuf(m.w, m.h)
	m.stampChat(buf, m.w)

	hintRow := m.h - 2
	msgRow := hintRow - 1

	// First char 'a' should be in name color, bold.
	px := buf[msgRow][0]
	if px.ch != 'a' {
		t.Errorf("want 'a', got %q", px.ch)
	}
	if !px.bold {
		t.Error("name char should be bold")
	}
	if px.r != 200 || px.g != 100 || px.b != 50 {
		t.Errorf("name char should have name color, got (%d,%d,%d)", px.r, px.g, px.b)
	}

	// Char after "alice: " (7 chars) should be text color, not bold.
	textPx := buf[msgRow][7]
	if textPx.ch != 'h' {
		t.Errorf("want 'h' at text start, got %q", textPx.ch)
	}
	if textPx.bold {
		t.Error("text char should not be bold")
	}
	if textPx.r != 210 {
		t.Errorf("text char should be grey (210), got r=%d", textPx.r)
	}
}

func TestStampChat_LongMessageWraps(t *testing.T) {
	longText := strings.Repeat("word ", 20) // 100 chars, far exceeds chatW-len("bob: ")
	m := Model{
		world: game.NewWorld(),
		w:     80, h: 24,
		chatMessages: []ChatMessage{
			{Name: "bob", R: 0, G: 200, B: 200, Text: longText, At: time.Now()},
		},
	}
	buf := makeTestBuf(m.w, m.h)
	m.stampChat(buf, m.w)

	hintRow := m.h - 2

	// With wrapping the message should occupy at least 2 rows.
	rowsWithContent := 0
	for row := 0; row < hintRow; row++ {
		for j := 0; j < 10; j++ {
			if buf[row][j].ch != ' ' {
				rowsWithContent++
				break
			}
		}
	}
	if rowsWithContent < 2 {
		t.Errorf("expected at least 2 message rows, got %d", rowsWithContent)
	}
}

func TestStampChat_MessageCappedAtThreeLines(t *testing.T) {
	// A very long single-message should produce at most maxLinesMsg=3 lines.
	longText := strings.Repeat("x ", 200) // 400 chars, wraps into many lines
	m := Model{
		world: game.NewWorld(),
		w:     80, h: 24,
		chatMessages: []ChatMessage{
			{Name: "a", R: 255, G: 255, B: 255, Text: longText, At: time.Now()},
		},
	}
	buf := makeTestBuf(m.w, m.h)
	m.stampChat(buf, m.w)

	hintRow := m.h - 2
	rowsWithContent := 0
	for row := 0; row < hintRow; row++ {
		for j := 0; j < 5; j++ {
			if buf[row][j].ch != ' ' {
				rowsWithContent++
				break
			}
		}
	}
	if rowsWithContent > 3 {
		t.Errorf("single message must produce at most 3 rows, got %d", rowsWithContent)
	}
}

func TestStampChat_HintRowPreservedWhenChatInactive(t *testing.T) {
	m := Model{
		world:    game.NewWorld(),
		w:        80,
		h:        24,
		chatMode: false,
	}
	buf := makeTestBuf(m.w, m.h)
	m.stampChat(buf, m.w)

	hintRow := m.h - 2
	// "↵ chat" should appear at hintRow.
	hint := "↵ chat"
	for i, expected := range hint {
		if buf[hintRow][i].ch != expected {
			t.Errorf("hintRow[%d]: want %q, got %q", i, expected, buf[hintRow][i].ch)
		}
	}
}

func TestStampChat_NoMessagesOnlyHint(t *testing.T) {
	m := Model{
		world:    game.NewWorld(),
		w:        80,
		h:        24,
		chatMode: false,
	}
	buf := makeTestBuf(m.w, m.h)
	m.stampChat(buf, m.w)

	hintRow := m.h - 2
	// All rows above hintRow should remain blank (no messages).
	for row := 0; row < hintRow; row++ {
		for j := 0; j < 10; j++ {
			if buf[row][j].ch != ' ' {
				t.Errorf("row %d col %d: expected blank, got %q", row, j, buf[row][j].ch)
			}
		}
	}
}
