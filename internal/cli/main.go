package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/eastriver/internkim/internal/fleetdomain"
)

var (
	boardUser        = "root"
	statusHTTPClient = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

const jetsonDefaultUser = "internkim"

type config struct {
	APIBaseURL     string
	RegisterSecret string
	CFDomain       string
}

func loadConfig() config {
	domain := envOr("INTERNKIM_DOMAIN", envOr("CLOUDFLARE_DOMAIN", fleetdomain.Default()))
	return config{
		APIBaseURL:     envOr("INTERNKIM_API_URL", fleetdomain.Subdomain("api", domain)),
		RegisterSecret: envOr("INTERNKIM_REGISTER_SECRET", ""),
		CFDomain:       domain,
	}
}

func currentExecutableFingerprint() string {
	executablePath, err := currentExecutablePath()
	if err != nil {
		return ""
	}
	file, err := os.Open(executablePath)
	if err != nil {
		return ""
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

func currentExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil {
		executablePath = resolvedPath
	}
	return executablePath, nil
}

func resolveRepositoryRootPath() (string, error) {
	workingDirectoryPath, errorValue := os.Getwd()
	if errorValue != nil {
		return "", errorValue
	}

	searchPath := workingDirectoryPath
	for {
		if _, errorValue := os.Stat(filepath.Join(searchPath, "go.mod")); errorValue == nil {
			return searchPath, nil
		}
		parentPath := filepath.Dir(searchPath)
		if parentPath == searchPath {
			return "", errors.New("could not find repository root")
		}
		searchPath = parentPath
	}
}

func Main() {
	requestedProfile, arguments := splitVaultProfileArgument(os.Args[1:])
	os.Args = append(os.Args[:1], arguments...)
	reExecuteWithVaultEnvironment(requestedProfile)
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	runNamedCommand(os.Args[1])
}

func runNamedCommand(name string) {
	switch name {
	case "--help", "-h":
		printUsage()
	case "setup":
		runSetup()
	case "flash":
		runFlash()
	case "wifi":
		runWiFi()
	case "ssh":
		runDeviceSSH()
	case "model":
		runModel()
	case "sync-tools":
		runSyncTools()
	case "migrate":
		runMigrate()
	case "invite":
		runInvite()
	case "users":
		runUsers()
	case "run":
		runTaskRun()
	case "reset":
		runReset()
	case "recover":
		runRecover()
	case "release":
		runRelease()
	case "status":
		runStatus()
	case "update":
		runUpdate()
	case "deploy":
		runDeploy()
	case "doctor":
		runDoctor()
	case "verify":
		runVerify()
	case "test":
		runTest()
	case "llm":
		runLLM()
	case "ops":
		runOps()
	case "dev":
		runDev()
	case "mac":
		runMac()
	case "lab":
		runLab()
	case "sim":
		runSim()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: internkim <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup    Full device provisioning")
	fmt.Println("  flash    Flash board boot media")
	fmt.Println("  wifi     Add or update Jetson Wi-Fi profiles")
	fmt.Println("  ssh      Open SSH to the device")
	fmt.Println("  model    Manage LLM model (current/set/list)")
	fmt.Println("  migrate  Migrate device metadata without full setup")
	fmt.Println("  invite   Add/invite an allowed user")
	fmt.Println("  users    Manage allowed users")
	fmt.Println("  task     Inspect task runs and failure logs")
	fmt.Println("  reset    Reset board runtime data")
	fmt.Println("  recover  Recover narrow device maintenance paths")
	fmt.Println("  release  Publish and inspect release sets")
	fmt.Println("  status   Check board and tunnel status")
	fmt.Println("  update   Deploy current build to device")
	fmt.Println("  deploy   Build and apply a signed release over Admin HTTPS")
	fmt.Println("  doctor   Check host dependencies")
	fmt.Println("  verify   Run API and browser verification")
	fmt.Println("  test     Run a prompt through disposable Local Fleet; use -o <file> for one returned attachment")
	fmt.Println("  llm      One-shot LLM ping (local by default, --remote for OpenRouter)")
	fmt.Println("  ops      Serve the local personal fleet console")
	fmt.Println("  mac      Install and run the Blueclaw guest on this Mac under vfkit")
	fmt.Println("  lab      Run container-based Blueclaw-aligned lab workflows")
	fmt.Println("  sim      Deprecated alias for lab")
}
