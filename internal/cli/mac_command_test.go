package cli

import (
	"runtime"
	"strings"
	"testing"
)

func TestMacRejectsASubcommandItDoesNotHave(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the command refuses a non-macOS host before it reaches the subcommand")
	}

	errorValue := runMacArguments([]string{"reinstall-everything"})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "unknown mac subcommand") {
		t.Fatalf("expected an unknown subcommand to be named, got %v", errorValue)
	}
}

func TestMacSaysWhyItCannotRunOnALinuxHost(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("this host is the one the command is for")
	}

	errorValue := runMacArguments([]string{"status"})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "macOS only") {
		t.Fatalf("expected the host to be named as the reason, got %v", errorValue)
	}
}

func TestTheRuntimeDirectoryLeavesRoomForAVSockSocket(t *testing.T) {
	layout, errorValue := macLayout("/somewhere/that/is/quite/long/indeed/and/then/some/more")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := layout.AssertVSockSocketPathFits(); errorValue != nil {
		t.Fatalf("the runtime directory is deliberately short so a long install root cannot break the boot: %v", errorValue)
	}
	if strings.HasPrefix(layout.RuntimeRootPath, layout.InstallRootPath) {
		t.Fatal("a runtime directory under the install root inherits its length, which is the failure this avoids")
	}
}
