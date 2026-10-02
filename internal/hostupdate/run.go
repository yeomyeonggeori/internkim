package hostupdate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const outputTailLineCount = 30

type Run struct {
	ToVersion         string
	InstallScriptPath string
	NotePath          string
	Machine           Machine
	Output            io.Writer
	Now               func() time.Time
	RunInstallScript  func(scriptPath string, arguments []string, output io.Writer) error
}

func NewRun(toVersion string, machine Machine) Run {
	return Run{
		ToVersion:         toVersion,
		InstallScriptPath: PackagedInstallScriptPath,
		NotePath:          NotePath,
		Machine:           machine,
		Output:            os.Stdout,
		Now:               time.Now,
		RunInstallScript:  runInstallScript,
	}
}

func InstallScriptArguments(toVersion string) []string {
	return []string{"host", "--channel", ChannelStable, "--version", toVersion}
}

func (run Run) Execute() error {
	captured := &bytes.Buffer{}
	scriptError := run.RunInstallScript(run.InstallScriptPath, InstallScriptArguments(run.ToVersion), io.MultiWriter(run.Output, captured))
	outcome := run.outcome(scriptError, captured.String())
	if errorValue := run.recordOutcome(outcome); errorValue != nil {
		return errorValue
	}
	if !outcome.Succeeded {
		return fmt.Errorf("the host update to %s did not finish: %s", run.ToVersion, outcome.Error)
	}
	return nil
}

func (run Run) outcome(scriptError error, output string) Outcome {
	installed := run.Machine.InstalledVersion()
	outcome := Outcome{FinishedAt: run.Now().UTC(), InstalledVersion: installed, OutputTail: lastLines(output, outputTailLineCount)}
	switch {
	case scriptError != nil:
		outcome.Error = "install.sh failed: " + scriptError.Error()
	case installed != run.ToVersion:
		outcome.Error = fmt.Sprintf("install.sh finished but the package manager reports %s installed", installed)
	default:
		outcome.Succeeded = true
	}
	return outcome
}

func (run Run) recordOutcome(outcome Outcome) error {
	note, isPending, errorValue := ReadNote(run.NotePath)
	if errorValue != nil || !isPending {
		return errorValue
	}
	note.Outcome = &outcome
	return WriteNote(run.NotePath, note)
}

func runInstallScript(scriptPath string, arguments []string, output io.Writer) error {
	command := exec.Command("sh", append([]string{scriptPath}, arguments...)...)
	command.Stdin = nil
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func lastLines(text string, count int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n")
}
