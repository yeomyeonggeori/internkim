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

func TestEveryRoomChangeTellsTheClients(t *testing.T) {
	closeCommand, errorValue := buzzCloseRoomsExceptCommand([]string{"광장"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	renameCommand, errorValue := buzzRenameRoomCommand("old::new")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	retireCommand, errorValue := buzzRetireRoomByNameCommand("old")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	for name, command := range map[string]string{"close": closeCommand, "rename": renameCommand, "retire": retireCommand} {
		if !strings.Contains(command, "DELETE FROM events WHERE kind IN (39000,39001,39002)") {
			t.Fatalf("%s changed rows without dropping the events a client reads: %s", name, command)
		}
		if !strings.Contains(command, "reconcile-channels") {
			t.Fatalf("%s dropped the events without writing them again: %s", name, command)
		}
	}
}
