package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/companyhost"
	"golang.org/x/term"
)

// thisComputer is the company host's own machine. Every command it runs is one
// the installer names; nothing here decides anything.
type thisComputer struct{}

func (thisComputer) Run(name string, arguments []string, environment []string, output io.Writer) error {
	command := exec.Command(name, arguments...)
	if environment != nil {
		command.Env = append(os.Environ(), environment...)
	}
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

// Output answers with what the command printed, and with an error carrying what
// it complained about, because a failure a person has to act on is in the
// program's own words rather than in an exit status.
func (thisComputer) Output(name string, arguments []string) (string, error) {
	command := exec.Command(name, arguments...)
	document, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return string(document), fmt.Errorf("%s: %s", errorValue, strings.TrimSpace(string(document)))
	}
	return string(document), nil
}

func (thisComputer) CarriesProgram(programName string) error {
	_, errorValue := exec.LookPath(programName)
	return errorValue
}

func (thisComputer) CarriesFile(path string) error {
	_, errorValue := os.Stat(path)
	return errorValue
}

type installArguments struct {
	ConnectionPath     string
	StateDirectoryPath string
	ModelKeyPath       string
}

func main() {
	command := filepath.Base(os.Args[0])
	if len(os.Args) < 2 || os.Args[1] != "install" {
		fmt.Fprintf(os.Stderr, "Usage: %s install <internkim-host.json> [--state-directory DIR] [--model-key-file FILE]\n", command)
		os.Exit(1)
	}
	if errorValue := runInstall(os.Args[2:]); errorValue != nil {
		fmt.Fprintf(os.Stderr, "\nInstallation stopped: %s\n", errorValue)
		os.Exit(1)
	}
}

func parseInstallArguments(arguments []string) (installArguments, error) {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	stateDirectoryPath := flags.String("state-directory", "", "persistent private installation directory")
	modelKeyPath := flags.String("model-key-file", "", "read the OpenRouter key from this private file")
	var connectionPaths []string
	remaining := arguments
	for {
		if errorValue := flags.Parse(remaining); errorValue != nil {
			return installArguments{}, errorValue
		}
		if flags.NArg() == 0 {
			break
		}
		connectionPaths = append(connectionPaths, flags.Arg(0))
		remaining = flags.Args()[1:]
	}
	if len(connectionPaths) != 1 {
		return installArguments{}, fmt.Errorf("name the internkim-host.json downloaded from company setup")
	}
	return installArguments{
		ConnectionPath:     connectionPaths[0],
		StateDirectoryPath: *stateDirectoryPath,
		ModelKeyPath:       *modelKeyPath,
	}, nil
}

func runInstall(arguments []string) error {
	parsed, errorValue := parseInstallArguments(arguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	modelKey, errorValue := readModelKeyFile(parsed.ModelKeyPath)
	if errorValue != nil {
		return errorValue
	}
	installation, errorValue := companyhost.Install(companyhost.Request{
		ConnectionPath:     parsed.ConnectionPath,
		StateDirectoryPath: parsed.StateDirectoryPath,
		ModelKey:           modelKey,
		PromptForModelKey:  func() (string, error) { return readModelKey(os.Stdin, os.Stdout) },
	}, thisComputer{}, os.Stdout)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("\nServer ready. Open %s/settings/setup and choose Check connection.\n", installation.Connection.AppURL)
	fmt.Printf("Private settings: %s\n%s starts the server when this computer starts. Keep it awake.\n",
		installation.StateDirectoryPath, installation.Supervisor)
	return nil
}

func readModelKeyFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", errorValue
	}
	key := strings.TrimSpace(string(document))
	if key == "" {
		return "", fmt.Errorf("%s holds no OpenRouter API key. Put the key from your account's Keys page in it", path)
	}
	return key, nil
}

func readModelKey(input *os.File, output io.Writer) (string, error) {
	if term.IsTerminal(int(input.Fd())) {
		fmt.Fprint(output, "OpenRouter API key (hidden): ")
		document, errorValue := term.ReadPassword(int(input.Fd()))
		fmt.Fprintln(output)
		return string(document), errorValue
	}
	line, errorValue := bufio.NewReader(input).ReadString('\n')
	if errorValue != nil && !errors.Is(errorValue, io.EOF) {
		return "", errorValue
	}
	if strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("no OpenRouter API key on standard input. Supply --model-key-file, or run this where you can type")
	}
	return line, nil
}
