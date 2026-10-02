package hostupdate

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ScriptRunner func(arguments []string, output io.Writer) error

func InstallScriptArguments(toVersion string) []string {
	return []string{PackagedInstallScriptPath, "host", "--channel", ChannelStable, "--version", toVersion}
}

func RunPackagedInstallScript(arguments []string, output io.Writer) error {
	command := exec.Command("sh", arguments...)
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func Update(notePath string, toVersion string, runScript ScriptRunner, now func() time.Time) error {
	captured := &bytes.Buffer{}
	scriptError := runScript(InstallScriptArguments(toVersion), io.MultiWriter(os.Stdout, captured))
	outcome := Outcome{FinishedAt: now().UTC(), Succeeded: scriptError == nil}
	if scriptError != nil {
		outcome.Error = "install.sh failed: " + scriptError.Error()
		outcome.OutputTail = lastLines(captured.String(), 30)
	}
	if errorValue := recordOutcome(notePath, outcome); errorValue != nil {
		return errorValue
	}
	return scriptError
}

func recordOutcome(notePath string, outcome Outcome) error {
	note, isPending, errorValue := ReadNote(notePath)
	if errorValue != nil || !isPending {
		return errorValue
	}
	note.Outcome = &outcome
	return WriteNote(notePath, note)
}

func lastLines(text string, count int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-count):], "\n")
}
