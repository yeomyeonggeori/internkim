package admind

import (
	"strings"
	"testing"
)

func TestARoomNameWithAnApostropheIsQuotedRatherThanBroken(t *testing.T) {
	quoted, errorValue := sqlQuotedRoomName("Dawn's room")
	if errorValue != nil {
		t.Fatalf("an apostrophe is an ordinary character in a room name: %v", errorValue)
	}
	if quoted != "'Dawn''s room'" {
		t.Fatalf("expected the apostrophe doubled, got %s", quoted)
	}
}

func TestARoomNameThatWouldEscapeTheShellIsRefused(t *testing.T) {
	for _, name := range []string{`say "hi"`, "back\\slash", "cost $5", "tick`s", "two\nlines"} {
		if _, errorValue := sqlQuotedRoomName(name); errorValue == nil {
			t.Fatalf("expected %q to be refused rather than mangled", name)
		}
	}
}

func TestARoomNameIsRequired(t *testing.T) {
	if _, errorValue := sqlQuotedRoomName("   "); errorValue == nil {
		t.Fatal("a name of only spaces names no room")
	}
}

func TestClosingEveryRoomKeepsTheOnesNamed(t *testing.T) {
	command, errorValue := buzzCloseRoomsExceptCommand([]string{"광장", "잡담", "welcome-everyone"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(command, "'광장', '잡담', 'welcome-everyone'") {
		t.Fatalf("expected the named rooms to stay open, got %s", command)
	}
	if !strings.Contains(command, "visibility = 'private'") {
		t.Fatalf("expected the rest to be closed, got %s", command)
	}
}

func TestRenamingARoomNeedsBothNames(t *testing.T) {
	if _, errorValue := buzzRenameRoomCommand("HR Compensation"); errorValue == nil {
		t.Fatal("a rename with no new name was accepted")
	}
	command, errorValue := buzzRenameRoomCommand("HR Compensation::HR")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(command, "SET name = 'HR' ") || !strings.Contains(command, "name = 'HR Compensation'") {
		t.Fatalf("expected the rename to name both, got %s", command)
	}
}
