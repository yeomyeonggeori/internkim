package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

func (thisComputer) Stream(name string, arguments []string, streams companyhost.Streams) error {
	command := exec.Command(name, arguments...)
	command.Stdin = streams.Input
	command.Stdout = streams.Output
	command.Stderr = streams.Errors
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
	if len(os.Args) < 2 {
		printUsage(command)
	}
	switch os.Args[1] {
	case "install":
		if errorValue := runInstall(os.Args[2:]); errorValue != nil {
			fmt.Fprintf(os.Stderr, "\nInstallation stopped: %s\n", errorValue)
			os.Exit(1)
		}
	case "box":
		runBox(os.Args[2:])
	case "backup":
		if errorValue := runBackup(os.Args[2:]); errorValue != nil {
			fmt.Fprintf(os.Stderr, "\nBackup stopped: %s\n", errorValue)
			os.Exit(1)
		}
	case "restore":
		if errorValue := runRestore(os.Args[2:]); errorValue != nil {
			fmt.Fprintf(os.Stderr, "\nRestore stopped: %s\n", errorValue)
			os.Exit(1)
		}
	case "import-device":
		if errorValue := runImportDevice(os.Args[2:]); errorValue != nil {
			fmt.Fprintf(os.Stderr, "\nImport stopped: %s\n", errorValue)
			os.Exit(1)
		}
	case "refresh":
		if errorValue := runRefresh(); errorValue != nil {
			fmt.Fprintf(os.Stderr, "\nThe company was not brought back: %s\n", errorValue)
			os.Exit(1)
		}
	default:
		printUsage(command)
	}
}

func printUsage(command string) {
	fmt.Fprintf(os.Stderr, "Usage: %s install <internkim-host.json> [--state-directory DIR] [--model-key-file FILE]\n", command)
	fmt.Fprintf(os.Stderr, "       %s box [--app-url URL]\n", command)
	fmt.Fprintf(os.Stderr, "       %s box code\n", command)
	fmt.Fprintf(os.Stderr, "       %s backup [--directory DIR] [--keep N]\n", command)
	fmt.Fprintf(os.Stderr, "       %s restore <archive> [--replace]\n", command)
	fmt.Fprintf(os.Stderr, "       %s import-device <migration-export-directory> --connection <internkim-host.json>\n", command)
	fmt.Fprintf(os.Stderr, "       %s refresh\n", command)
	os.Exit(1)
}

func runBox(arguments []string) {
	if len(arguments) > 0 && arguments[0] == "code" {
		printPairingCode()
		return
	}
	flags := flag.NewFlagSet("box", flag.ExitOnError)
	appURL := flags.String("app-url", blueclaw.CompanyPackageHomepage, "the address this company signs in at, which a box announces itself to")
	flags.Parse(arguments)
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errorValue := boxDaemon(*appURL).Run(ctx)
	if errors.Is(errorValue, box.ErrConnectedByFile) {
		fmt.Println(errorValue)
		return
	}
	if errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}

func printPairingCode() {
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
	shown, isLive, errorValue := box.ShownPairingCode(blueclaw.CompanyHostBoxStatePath, time.Now())
	if errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
	if !isLive {
		fmt.Fprintf(os.Stderr, "This box shows no code right now. Check that %s is running; a new code arrives within a minute.\n", blueclaw.BoxServiceName)
		os.Exit(1)
	}
	fmt.Printf("%s\nEnter it on the company setup page before %s.\n", shown.Code, shown.ExpiresAt.Local().Format("15:04"))
}

func boxDaemon(appURL string) box.Daemon {
	return box.Daemon{
		Client: box.Client{AppURL: appURL},
		Places: box.Places{
			StateDirectoryPath:        blueclaw.CompanyHostBoxStatePath,
			PairingPageListenAddress:  blueclaw.CompanyHostBoxPairingPageListenAddress,
			ConnectionFilePath:        companyhost.CurrentConnectionPath(),
			CredentialPaths:           []string{blueclaw.CompanyHostAgentKeyPath, blueclaw.RelayAgentKeyPath},
			ModelKeyPath:              blueclaw.CompanyHostModelKeyPath,
			CompanyStateDirectoryPath: companyhost.DefaultStateDirectoryPath,
		},
		Install: func(request companyhost.Request) error {
			_, errorValue := companyhost.Install(request, thisComputer{}, os.Stdout)
			return errorValue
		},
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

func runRefresh() error {
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	if errorValue := companyhost.Refresh(thisComputer{}, os.Stdout); errorValue != nil {
		return errorValue
	}
	fmt.Println("\nServer ready on this release.")
	return nil
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
	if companyhost.ThisMachineKeepsABoxSessionFresh() {
		return installByClaiming(parsed, modelKey)
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

func installByClaiming(parsed installArguments, modelKey string) error {
	connection, errorValue := companyhost.ReadConnection(parsed.ConnectionPath)
	if errorValue != nil {
		return errorValue
	}
	if modelKey == "" {
		modelKey, errorValue = readModelKey(os.Stdin, os.Stdout)
		if errorValue != nil {
			return errorValue
		}
	}
	daemon := boxDaemon(connection.AppURL)
	if parsed.StateDirectoryPath != "" {
		stateDirectoryPath, errorValue := filepath.Abs(parsed.StateDirectoryPath)
		if errorValue != nil {
			return errorValue
		}
		daemon.Places.CompanyStateDirectoryPath = func(string) string { return stateDirectoryPath }
	}
	if errorValue := daemon.InstallWithConnectionFile(context.Background(), connection.AgentKey, strings.TrimSpace(modelKey)); errorValue != nil {
		return errorValue
	}
	if errorValue := companyhost.KeepTheBoxSessionFresh(thisComputer{}, os.Stdout); errorValue != nil {
		return errorValue
	}
	fmt.Printf("\nServer ready. Open %s/settings/setup and choose Check connection.\n", connection.AppURL)
	fmt.Println("The connection file is spent: this computer now proves itself with its own key, and systemd keeps it signed in.")
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
