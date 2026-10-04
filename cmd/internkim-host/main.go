package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/boxwifi"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"golang.org/x/term"
)

const (
	planeProbeTimeout = 5 * time.Second
	passiveRescanMode = "no"
)

// thisComputer is the company host's own machine. Every command it runs is one
// the installer names; nothing here decides anything.
type thisComputer struct {
	companyhost.LocalProcesses
}

func (thisComputer) Run(name string, arguments []string, environment []string, output io.Writer) error {
	command := exec.Command(name, arguments...)
	if environment != nil {
		command.Env = append(os.Environ(), environment...)
	}
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

// Output answers with what the command printed on its standard output, and with
// an error carrying what it complained about, because a failure a person has to
// act on is in the program's own words rather than in an exit status.
func (thisComputer) Output(name string, arguments []string) (string, error) {
	var printed, complaint bytes.Buffer
	command := exec.Command(name, arguments...)
	command.Stdout = &printed
	command.Stderr = &complaint
	if errorValue := command.Run(); errorValue != nil {
		return printed.String(), fmt.Errorf("%s: %s", errorValue, strings.TrimSpace(complaint.String()+"\n"+printed.String()))
	}
	return printed.String(), nil
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
	ConnectionPath string
	ModelKeyPath   string
}

func main() {
	command := filepath.Base(os.Args[0])
	if len(os.Args) < 2 {
		printUsage(command)
	}
	switch os.Args[1] {
	case "install":
		stopOnFailure("Installation stopped", runInstall(os.Args[2:]))
	case "box":
		runBox(os.Args[2:])
	case "backup":
		stopOnFailure("Backup stopped", runBackup(os.Args[2:]))
	case "restore":
		stopOnFailure("Restore stopped", runRestore(os.Args[2:]))
	case "refresh":
		stopOnFailure("The company was not brought back", runRefresh())
	case blueclaw.SkillPreparationVerb:
		if len(os.Args) > 2 {
			printUsage(command)
		}
		stopOnFailure("The skills were not prepared", companyhost.PrepareTheBundledSkills(thisComputer{}, os.Stdout))
	case "update":
		stopOnFailure("Update stopped", runUpdate(os.Args[2:]))
	default:
		printUsage(command)
	}
}

func stopOnFailure(whatStopped string, errorValue error) {
	if errorValue == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s: %s\n", whatStopped, errorValue)
	os.Exit(1)
}

func printUsage(command string) {
	fmt.Fprintf(os.Stderr, "Usage: %s install <internkim-host.json> [--model-key-file FILE]\n", command)
	fmt.Fprintf(os.Stderr, "       %s box [--app-url URL] [--wifi-setup] [--host-name-from-company]\n", command)
	fmt.Fprintf(os.Stderr, "       %s box code\n", command)
	fmt.Fprintf(os.Stderr, "       %s backup [--directory DIR] [--keep N]\n", command)
	fmt.Fprintf(os.Stderr, "       %s restore <archive> [--replace]\n", command)
	fmt.Fprintf(os.Stderr, "       %s refresh\n", command)
	fmt.Fprintf(os.Stderr, "       %s %s\n", command, blueclaw.SkillPreparationVerb)
	fmt.Fprintf(os.Stderr, "       %s update --version vYYYY.MM.DD.HHMMSS\n", command)
	os.Exit(1)
}

func runBox(arguments []string) {
	if len(arguments) > 0 && arguments[0] == "code" {
		printPairingCode()
		return
	}
	flags := flag.NewFlagSet("box", flag.ExitOnError)
	appURL := flags.String("app-url", blueclaw.CompanyPackageHomepage, "the address this company signs in at, which a box announces itself to")
	setsUpWifi := flags.Bool("wifi-setup", false, "while this box is empty and offline, open the kimmini network and ask for the office Wi-Fi")
	namesHostFromCompany := flags.Bool("host-name-from-company", false, "name this computer after the company it belongs to, and kimmini while it belongs to none, as a Kim mini is named")
	madeOnPath := flags.String("made-on-file", boxwifi.DefaultMadeOnPath, "a file holding the day this box was made as YYYY-MM-DD, added to the kimmini network's name")
	flags.Parse(arguments)
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	daemon := boxDaemon(*appURL)
	if *setsUpWifi {
		daemon = withWifiSetup(daemon, *appURL, madeOnFrom(*madeOnPath))
	}
	if *namesHostFromCompany {
		daemon = withHostNameFromCompany(daemon)
	}
	errorValue := daemon.Run(ctx)
	if errors.Is(errorValue, box.ErrConnectedByFile) {
		fmt.Println(errorValue)
		return
	}
	if errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}

func madeOnFrom(path string) time.Time {
	madeOn, errorValue := boxwifi.ReadMadeOn(path)
	if errorValue != nil {
		log.Printf("naming the kimmini network without a date: %v", errorValue)
	}
	return madeOn
}

func withWifiSetup(daemon box.Daemon, appURL string, madeOn time.Time) box.Daemon {
	radio := boxwifi.NetworkManagerRadio{ReachesPlane: func(ctx context.Context) bool { return reachesURL(ctx, appURL) }}
	watcherRadio := radio
	watcherRadio.RescanMode = passiveRescanMode
	daemon.GetOnline = func(ctx context.Context, boxPublicKey string) error {
		setup := boxwifi.Setup{Radio: radio, NetworkName: boxwifi.SetupNetworkNameFor(madeOn)}
		return setup.Run(ctx)
	}
	daemon.ChangeWifi = radio.Switch
	daemon.ScanWifi = func(ctx context.Context) ([]box.NearbyNetwork, error) {
		return scanNearbyNetworks(ctx, watcherRadio)
	}
	return daemon
}

func withHostNameFromCompany(daemon box.Daemon) box.Daemon {
	daemon.NameHost = boxwifi.HostNamer{}.Name
	return daemon
}

func reachesURL(ctx context.Context, address string) bool {
	probeContext, cancel := context.WithTimeout(ctx, planeProbeTimeout)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(probeContext, http.MethodGet, address, nil)
	if errorValue != nil {
		return false
	}
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		return false
	}
	response.Body.Close()
	return true
}

func scanNearbyNetworks(ctx context.Context, radio boxwifi.NetworkManagerRadio) ([]box.NearbyNetwork, error) {
	scanned, errorValue := radio.Scan(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	nearby := make([]box.NearbyNetwork, 0, len(scanned))
	for _, network := range scanned {
		nearby = append(nearby, box.NearbyNetwork{SSID: network.SSID, SignalPercent: network.SignalPercent, IsSecured: network.IsSecured, IsConnected: network.IsConnected})
	}
	return nearby, nil
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
		ConnectionPath: connectionPaths[0],
		ModelKeyPath:   *modelKeyPath,
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

func runUpdate(arguments []string) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	version := flags.String("version", "", "the stable release to install, as its tag")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(*version) == "" || flags.NArg() != 0 {
		return fmt.Errorf("name the release to install with --version, as its tag")
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	return hostupdate.Update(hostupdate.NotePath, hostupdate.TagOf(*version), hostupdate.RunPackagedInstallScript, time.Now)
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
	connection, errorValue := companyhost.ReadConnection(parsed.ConnectionPath)
	if errorValue != nil {
		return errorValue
	}
	if companyhost.ThisMachineKeepsABoxSessionFresh() {
		return installByClaiming(parsed, connection, modelKey)
	}
	installation, errorValue := companyhost.Install(companyhost.Request{
		Connection:        connection,
		ModelKey:          modelKey,
		PromptForModelKey: func() (string, error) { return readModelKey(os.Stdin, os.Stdout) },
	}, thisComputer{}, os.Stdout)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("\nServer ready. Open %s/settings/setup and choose Check connection.\n", installation.Connection.AppURL)
	fmt.Printf("Private settings: %s\n%s starts the server when this computer starts. Keep it awake.\n",
		installation.StateDirectoryPath, installation.Supervisor)
	return nil
}

func installByClaiming(parsed installArguments, connection companyhost.Connection, modelKey string) error {
	if modelKey == "" {
		var errorValue error
		modelKey, errorValue = readModelKey(os.Stdin, os.Stdout)
		if errorValue != nil {
			return errorValue
		}
	}
	daemon := boxDaemon(connection.AppURL)
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
