package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type buzzPublicHostConnection struct {
	recordedPublicURL string
	commands          []string
}

// A device that is configured answers yes to everything the step checks, so a
// test that wants one failing check gets it on its own.
func (connection *buzzPublicHostConnection) Run(command string) string {
	connection.commands = append(connection.commands, command)
	switch {
	case strings.HasPrefix(command, "cat ") && strings.Contains(command, blueclaw.BuzzRelayPublicURLFilePath):
		return connection.recordedPublicURL
	case strings.Contains(command, "echo y || echo n"):
		return "y"
	case strings.HasPrefix(command, "systemctl is-active"):
		return "active"
	}
	return ""
}

func (connection *buzzPublicHostConnection) SCP(localPath, remotePath string) error {
	return nil
}

func TestBuzzPublicHostLeavesTheRelayOnLoopbackWithoutADomain(t *testing.T) {
	connection := &buzzPublicHostConnection{}
	context := &Context{Backend: BackendSSH, SSH: connection, PublicURL: "https://a-device.example.test"}
	if !StepBuzzPublicHost.IsSatisfied(context) {
		t.Fatal("a company that chose no relay domain has nothing to configure")
	}
	if errorValue := StepBuzzPublicHost.Run(context); errorValue != nil {
		t.Fatalf("run without a relay domain: %v", errorValue)
	}
	if len(connection.commands) != 0 {
		t.Fatalf("nothing may be changed on the device: %v", connection.commands)
	}
}

func TestBuzzPublicHostRekeysTheCommunityFromTheNameItRecorded(t *testing.T) {
	connection := &buzzPublicHostConnection{recordedPublicURL: "wss://old.example.test"}
	context := &Context{Backend: BackendSSH, SSH: connection, RelayDomain: "new.example.test"}
	if errorValue := StepBuzzPublicHost.Run(context); errorValue != nil {
		t.Fatalf("run with a relay domain: %v", errorValue)
	}
	rekey := ""
	for _, command := range connection.commands {
		if strings.Contains(command, "UPDATE communities") {
			rekey = command
		}
	}
	if rekey == "" {
		t.Fatal("the community must be re-keyed to the chosen domain")
	}
	if !strings.Contains(rekey, "host='new.example.test'") {
		t.Fatalf("re-key must set the chosen domain: %s", rekey)
	}
	if !strings.Contains(rekey, "'old.example.test'") {
		t.Fatalf("re-key must reach the row the previous name left behind: %s", rekey)
	}
	if !strings.Contains(rekey, "'"+blueclaw.BuzzRelayBindAddress+"'") {
		t.Fatalf("re-key must still reach a relay that never had a domain: %s", rekey)
	}
}

func TestBuzzPublicHostIsUnsatisfiedWhenTheRecordedNameIsStale(t *testing.T) {
	connection := &buzzPublicHostConnection{recordedPublicURL: "wss://old.example.test"}
	context := &Context{Backend: BackendSSH, SSH: connection, RelayDomain: "new.example.test"}
	if StepBuzzPublicHost.IsSatisfied(context) {
		t.Fatal("a device still carrying the previous relay name must be reconfigured")
	}
	connection.recordedPublicURL = "wss://new.example.test"
	if !StepBuzzPublicHost.IsSatisfied(context) {
		t.Fatal("a device already answering on the chosen domain must be left alone")
	}
}
