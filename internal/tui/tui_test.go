package tui

import (
	"testing"

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
