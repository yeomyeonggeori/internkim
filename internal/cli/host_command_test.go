package cli

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type recordingHostCommandExecutor struct {
	path       string
	runErrors  []error
	runs       []hostCommandInvocation
	outputs    []hostCommandInvocation
	outputText map[string]string
}

func (executor *recordingHostCommandExecutor) LookPath(name string) (string, error) {
	if name != "container" {
		return "", errors.New("unexpected executable: " + name)
	}
	if executor.path == "" {
		return "", errors.New("missing executable")
	}
	return executor.path, nil
}

func (executor *recordingHostCommandExecutor) CombinedOutput(invocation hostCommandInvocation) ([]byte, error) {
	executor.outputs = append(executor.outputs, copyHostCommandInvocation(invocation))
	if value, ok := executor.outputText[hostCommandInvocationKey(invocation)]; ok {
		return []byte(value), nil
	}
	return []byte{}, nil
}

func (executor *recordingHostCommandExecutor) Run(invocation hostCommandInvocation) error {
	executor.runs = append(executor.runs, copyHostCommandInvocation(invocation))
	if len(executor.runErrors) == 0 {
		return nil
	}
	errorValue := executor.runErrors[0]
	executor.runErrors = executor.runErrors[1:]
	return errorValue
}

func TestParseHostAddTeamOptionsForwardsRemoteArguments(t *testing.T) {
	options, errorValue := parseHostAddTeamOptions([]string{
		"--team", "pilot-01",
		"--display-name", "Pilot",
		"--member", "owner@example.com:Owner:owner-password",
		"--vm", "vm-2",
		"--gateway-url", "https://gateway.example.com/v1/chat/completions",
		"--systemd-dir", "/etc/systemd/system",
		"--port-base", "22000",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if options.TeamID != "pilot-01" || options.VirtualMachineName != "vm-2" {
		t.Fatalf("unexpected host add-team options: %+v", options)
	}
	expectedArguments := []string{
		"--display-name", "Pilot",
		"--member", "owner@example.com:Owner:owner-password",
		"--gateway-url", "https://gateway.example.com/v1/chat/completions",
		"--systemd-dir", "/etc/systemd/system",
		"--port-base", "22000",
	}
	if !reflect.DeepEqual(options.RemoteArguments, expectedArguments) {
		t.Fatalf("unexpected remote arguments:\nwant: %#v\n got: %#v", expectedArguments, options.RemoteArguments)
	}
}

func TestHostAddTeamBuildsExactRemoteContainerExec(t *testing.T) {
	executor := &recordingHostCommandExecutor{path: "container"}
	exitCode, errorValue := executeHostAddTeam([]string{
		"--team", "pilot-01",
		"--display-name", "Pilot",
		"--member", "owner@example.com:Owner:owner-password",
		"--vm", "vm-2",
		"--gateway-url", "https://gateway.example.com/v1/chat/completions",
		"--systemd-dir", "/etc/systemd/system",
		"--port-base", "22000",
	}, executor, io.Discard, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(executor.runs) != 3 {
		t.Fatalf("expected build, install, and add-team runs, got %d", len(executor.runs))
	}
	expectedArguments := []string{
		"exec",
		"vm-2",
		"internkim",
		"tenant",
		"provision",
		"--runtime",
		"host",
		"--tenant",
		"pilot-01",
		"--display-name",
		"Pilot",
		"--member",
		"owner@example.com:Owner:owner-password",
		"--gateway-url",
		"https://gateway.example.com/v1/chat/completions",
		"--systemd-dir",
		"/etc/systemd/system",
		"--port-base",
		"22000",
	}
	assertHostRun(t, executor.runs[2], "container", expectedArguments)
}

func TestHostRemoveTeamBuildsExactRemoteContainerExec(t *testing.T) {
	executor := &recordingHostCommandExecutor{path: "container"}
	exitCode, errorValue := executeHostRemoveTeam([]string{
		"--team", "pilot-01",
		"--vm", "vm-2",
		"--",
		"--account-id", "account-id",
		"--tunnel-id", "tunnel-id",
		"--api-token-path", "/tmp/token",
		"--hostname-template", "{tenant}.example.com",
		"--force",
	}, executor, io.Discard, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(executor.runs) != 3 {
		t.Fatalf("expected build, install, and remove-team runs, got %d", len(executor.runs))
	}
	expectedArguments := []string{
		"exec",
		"vm-2",
		"internkim",
		"tenant",
		"remove",
		"--tenant",
		"pilot-01",
		"--confirm",
		"pilot-01",
		"--account-id",
		"account-id",
		"--tunnel-id",
		"tunnel-id",
		"--api-token-path",
		"/tmp/token",
		"--hostname-template",
		"{tenant}.example.com",
		"--force",
	}
	assertHostRun(t, executor.runs[2], "container", expectedArguments)
}

func TestHostSyncCLIBuildsExactInstallContainerExec(t *testing.T) {
	executor := &recordingHostCommandExecutor{path: "container"}
	errorValue := executeHostSyncCLI([]string{"--vm", "vm-1"}, executor, io.Discard, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(executor.runs) != 2 {
		t.Fatalf("expected build and install runs, got %d", len(executor.runs))
	}
	build := executor.runs[0]
	if build.ExecutableName != "go" {
		t.Fatalf("unexpected build executable: %+v", build)
	}
	if len(build.Arguments) != 4 || build.Arguments[0] != "build" || build.Arguments[1] != "-o" || build.Arguments[3] != "./cmd/internkim" {
		t.Fatalf("unexpected build arguments: %+v", build.Arguments)
	}
	if !reflect.DeepEqual(build.Environment, []string{"GOOS=linux", "GOARCH=arm64"}) {
		t.Fatalf("unexpected build environment: %+v", build.Environment)
	}
	expectedArguments := []string{
		"exec",
		"--interactive",
		"vm-1",
		"sh",
		"-c",
		"cat > /usr/local/bin/internkim && chmod 755 /usr/local/bin/internkim",
	}
	assertHostRun(t, executor.runs[1], "container", expectedArguments)
}

func TestExecuteHostCommandDispatchesSyncCLI(t *testing.T) {
	executor := &recordingHostCommandExecutor{path: "container"}
	exitCode, errorValue := executeHostCommand("sync-cli", []string{"--vm", "vm-1"}, executor)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(executor.runs) != 2 {
		t.Fatalf("expected sync-cli dispatch to run build and install, got %d", len(executor.runs))
	}
}

func TestHostInitSkipsCreationWhenVirtualMachineIsRunning(t *testing.T) {
	executor := &recordingHostCommandExecutor{path: "container"}
	errorValue := executeHostInit([]string{"--vm", "vm-1"}, executor, io.Discard, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, run := range executor.runs {
		if len(run.Arguments) > 0 && run.Arguments[0] == "run" {
			t.Fatalf("expected no VM creation for a running VM, got %+v", run.Arguments)
		}
	}
	if len(executor.runs) != 2 {
		t.Fatalf("expected sync-cli build and install only, got %d", len(executor.runs))
	}
}

func TestHostCreateVirtualMachineInvocationArguments(t *testing.T) {
	invocation := hostCreateVirtualMachineInvocation("container", hostInitOptions{
		VirtualMachineName: "vm-9",
		CPUCount:           "8",
		Memory:             "12g",
		Image:              "docker.io/library/ubuntu:24.04",
	}, io.Discard, io.Discard)
	expectedPrefix := []string{
		"run",
		"--detach",
		"--name", "vm-9",
		"--cpus", "8",
		"--memory", "12g",
		"--virtualization",
		"docker.io/library/ubuntu:24.04",
		"sh", "-c",
	}
	if !reflect.DeepEqual(invocation.Arguments[:len(expectedPrefix)], expectedPrefix) {
		t.Fatalf("unexpected create arguments: %+v", invocation.Arguments)
	}
	bootstrapScript := invocation.Arguments[len(invocation.Arguments)-1]
	for _, fragment := range []string{"exec /lib/systemd/systemd", "systemd-sysv", "sudoers.d/admin", "resolv.conf"} {
		if !strings.Contains(bootstrapScript, fragment) {
			t.Fatalf("bootstrap script is missing %q", fragment)
		}
	}
}

func assertHostRun(t *testing.T, invocation hostCommandInvocation, expectedExecutable string, expectedArguments []string) {
	t.Helper()
	if invocation.ExecutableName != expectedExecutable {
		t.Fatalf("unexpected executable: want %q got %q", expectedExecutable, invocation.ExecutableName)
	}
	if !reflect.DeepEqual(invocation.Arguments, expectedArguments) {
		t.Fatalf("unexpected arguments:\nwant: %#v\n got: %#v", expectedArguments, invocation.Arguments)
	}
}

func copyHostCommandInvocation(invocation hostCommandInvocation) hostCommandInvocation {
	return hostCommandInvocation{
		ExecutableName: invocation.ExecutableName,
		Arguments:      append([]string{}, invocation.Arguments...),
		Stdin:          invocation.Stdin,
		Stdout:         invocation.Stdout,
		Stderr:         invocation.Stderr,
		Environment:    append([]string{}, invocation.Environment...),
	}
}

func hostCommandInvocationKey(invocation hostCommandInvocation) string {
	return invocation.ExecutableName + "\x00" + strings.Join(invocation.Arguments, "\x00")
}
