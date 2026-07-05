package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type verifyTarget struct {
	host       string
	user       string
	password   string
	nodeID     string
	scriptDir  string
	stateDir   string
	sshpassBin string
	sshClient  *sshClient
}

func runVerify() {
	if errorValue := runVerifyArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runVerifyArguments(arguments []string) error {
	subcommand := "api"
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		subcommand = arguments[0]
		arguments = arguments[1:]
	}

	switch subcommand {
	case "api":
		return runVerifyAPI(arguments)
	case "mattermost":
		return runVerifyMattermost(arguments)
	case "site":
		return runVerifySite(arguments)
	case "browser":
		return runVerifyBrowser(arguments)
	default:
		return fmt.Errorf("unknown verify subcommand: %s", subcommand)
	}
}

func runVerifyAPI(arguments []string) error {
	verifyTarget, errorValue := resolveVerifyTarget(arguments)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("verify api: %s@%s\n", verifyTarget.user, verifyTarget.host)
	return verifyTarget.runRemoteVerification(verifyAPIScript())
}

func runVerifyMattermost(arguments []string) error {
	flagSet := flag.NewFlagSet("verify mattermost", flag.ContinueOnError)
	prompt := flagSet.String("prompt", "", "Post this prompt through the real Mattermost ingress path")
	expectBrowserOpen := flagSet.Bool("expect-browser-open", false, "Require a successful browser.open tool result for prompt verification")
	expectPublicURL := flagSet.Bool("expect-public-url", false, "Require a public URL in the final bot reply and verify it returns site HTML")
	htmlAttachmentFollowupE2E := flagSet.Bool("html-attachment-followup-e2e", false, "Upload an HTML file through Mattermost and verify current and follow-up attachment preview")
	messageDeleteE2E := flagSet.Bool("message-delete-e2e", false, "Create Mattermost test posts and verify InternKim deletes only its own posts")
	directMessageE2E := flagSet.Bool("direct-message-e2e", false, "Create Mattermost probe users and verify message.send sends a direct message")
	expectedTools := repeatedStringFlag{}
	expectedEvents := repeatedStringFlag{}
	flagSet.Var(&expectedTools, "expect-tool", "Require a requested tool event for prompt verification; repeat for multiple tools")
	flagSet.Var(&expectedEvents, "expect-event", "Require a task event name for prompt verification; repeat for multiple events")
	browserOpenE2E := flagSet.Bool("browser-open-e2e", false, "Pair a local probe companion and require a successful browser.open result")
	keep := flagSet.Bool("keep", false, "Keep probe messages and users for inspection")
	keepBrowser := flagSet.Bool("keep-browser", false, "Keep the local browser window open after browser-open E2E")
	downloadFilesTo := flagSet.String("download-files-to", "", "Download final Mattermost bot attachments into this local directory")
	waitForCompletion := flagSet.Bool("wait-for-completion", false, "Wait for the created task to complete before reading the final Mattermost bot reply")
	timeoutSeconds := flagSet.Int("timeout", 240, "Seconds to wait for the bot reply")
	companionPath := flagSet.String("companion-path", "/Applications/Intern Kim Companion.app/Contents/MacOS/internkim-companion", "Local internkim-companion executable for browser-open E2E")
	agentBrowserPath := flagSet.String("agent-browser-path", "/Applications/Intern Kim Companion.app/Contents/MacOS/agent-browser", "Local agent-browser executable for browser-open E2E")
	target := registerTargetFlags(flagSet)
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(*prompt) != "" && isMattermostSiteVerification(*expectPublicURL, expectedTools.Values()) && !hasExplicitVerifyTargetArgument(arguments) {
		return errors.New("Mattermost site verification defaults to the saved physical board; use `internkim test \"<prompt>\" --expect-public-url` for Local Fleet verification or pass an explicit target")
	}

	verifyTarget, errorValue := target.resolveVerifyTarget()
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("verify mattermost: %s@%s\n", verifyTarget.user, verifyTarget.host)
	if *htmlAttachmentFollowupE2E {
		return verifyTarget.runRemoteVerificationWithTimeout(verifyMattermostHTMLAttachmentFollowupScript(*keep, *timeoutSeconds), mattermostPromptSSHTimeout(*timeoutSeconds))
	}
	if *messageDeleteE2E {
		return verifyTarget.runRemoteVerificationWithTimeout(verifyMattermostMessageDeleteE2EScript(*keep, *timeoutSeconds), mattermostPromptSSHTimeout(*timeoutSeconds))
	}
	if *directMessageE2E {
		return verifyTarget.runRemoteVerificationWithTimeout(verifyMattermostDirectMessageE2EScript(*keep, *timeoutSeconds), mattermostPromptSSHTimeout(*timeoutSeconds))
	}
	if *browserOpenE2E {
		promptText := strings.TrimSpace(*prompt)
		if promptText == "" {
			promptText = "브라우저 열어줘."
		}
		return runMattermostBrowserOpenE2E(verifyTarget, promptText, *keep, *keepBrowser, *timeoutSeconds, *companionPath, *agentBrowserPath)
	}
	if strings.TrimSpace(*prompt) != "" {
		return verifyTarget.runMattermostPromptVerification(
			verifyMattermostPromptScript(*prompt, *keep, *timeoutSeconds, *expectBrowserOpen, *expectPublicURL, expectedTools.Values(), expectedEvents.Values(), strings.TrimSpace(*downloadFilesTo) != "", *waitForCompletion, false),
			mattermostPromptScriptSSHTimeout(*timeoutSeconds, *expectPublicURL),
			strings.TrimSpace(*downloadFilesTo),
		)
	}
	if *expectBrowserOpen || *expectPublicURL || len(expectedTools.Values()) > 0 || len(expectedEvents.Values()) > 0 {
		return fmt.Errorf("--expect-browser-open, --expect-public-url, --expect-tool, and --expect-event require --prompt")
	}
	return verifyTarget.runRemoteVerificationWithTimeout(verifyMattermostScript(), 6*time.Minute)
}

const defaultVerifySitePrompt = "테스트용 'Local Fleet Studio' 단일 페이지 소개 웹사이트를 만들어서 배포해줘. 첫 화면 제목은 'Local Fleet Studio', 보조 문구는 '로컬 플릿 웹사이트 생성 배포 테스트', 섹션은 서비스 소개, 장점 3개, 문의 CTA만 넣어줘. 추가 질문하지 말고 합리적인 기본값으로 진행해줘."

func runVerifySite(arguments []string) error {
	flagSet := flag.NewFlagSet("verify site", flag.ContinueOnError)
	prompt := flagSet.String("prompt", defaultVerifySitePrompt, "Post this site creation prompt through Mattermost")
	keep := flagSet.Bool("keep", false, "Keep probe messages, users, and site for inspection")
	timeoutSeconds := flagSet.Int("timeout", 420, "Seconds to wait for the site deployment task")
	target := registerTargetFlags(flagSet)
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if !hasExplicitVerifyTargetArgument(arguments) {
		return errors.New("verify site defaults to the saved physical board; use `internkim test \"<prompt>\" --expect-public-url` for Local Fleet verification or pass an explicit target")
	}

	verifyTarget, errorValue := target.resolveVerifyTarget()
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("verify site: %s@%s\n", verifyTarget.user, verifyTarget.host)
	expectedTools := []string{"site.create", "terminal.run", "site.publish"}
	return verifyTarget.runRemoteVerificationWithTimeout(verifyMattermostPromptScript(*prompt, *keep, *timeoutSeconds, false, true, expectedTools, nil, false, false, false), mattermostPromptScriptSSHTimeout(*timeoutSeconds, true))
}

func isMattermostSiteVerification(expectPublicURL bool, expectedTools []string) bool {
	if expectPublicURL {
		return true
	}
	for _, toolName := range expectedTools {
		if strings.HasPrefix(strings.TrimSpace(toolName), "site.") {
			return true
		}
	}
	return false
}

func hasExplicitVerifyTargetArgument(arguments []string) bool {
	for _, flagName := range []string{"--host", "--node", "--board", "--cloudflare-ssh", "--sim"} {
		if hasCommandArgument(arguments, flagName) {
			return true
		}
	}
	return false
}

func verifyTargetArguments(host string, user string, password string, node string, cloudflareSSH bool, board string, simulation bool) []string {
	targetArguments := []string{}
	if strings.TrimSpace(host) != "" {
		targetArguments = append(targetArguments, "--host", strings.TrimSpace(host))
	}
	if strings.TrimSpace(user) != "" {
		targetArguments = append(targetArguments, "--user", strings.TrimSpace(user))
	}
	if strings.TrimSpace(password) != "" {
		targetArguments = append(targetArguments, "--password", password)
	}
	if strings.TrimSpace(node) != "" {
		targetArguments = append(targetArguments, "--node", strings.TrimSpace(node))
	}
	if cloudflareSSH {
		targetArguments = append(targetArguments, "--cloudflare-ssh")
	}
	if strings.TrimSpace(board) != "" {
		targetArguments = append(targetArguments, "--board", strings.TrimSpace(board))
	}
	if simulation {
		targetArguments = append(targetArguments, "--sim")
	}
	return targetArguments
}

type repeatedStringFlag struct {
	values []string
}

func (flagValue *repeatedStringFlag) String() string {
	return strings.Join(flagValue.values, ",")
}

func (flagValue *repeatedStringFlag) Set(value string) error {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue != "" {
		flagValue.values = append(flagValue.values, trimmedValue)
	}
	return nil
}

func (flagValue repeatedStringFlag) Values() []string {
	return append([]string{}, flagValue.values...)
}

func trimmedNonEmptyValues(values []string) []string {
	trimmedValues := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			trimmedValues = append(trimmedValues, trimmedValue)
		}
	}
	return trimmedValues
}

type mattermostBrowserOpenE2EPreparation struct {
	DeviceURL string `json:"deviceURL"`
	Code      string `json:"code"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	UserID    string `json:"userID"`
	ChannelID string `json:"channelID"`
}

func runMattermostBrowserOpenE2E(target verifyTarget, prompt string, keep bool, keepBrowser bool, timeoutSeconds int, companionPath string, agentBrowserPath string) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	preparationOutput, errorValue := target.sshClient.runResult(prepareMattermostBrowserOpenE2EScript())
	if strings.TrimSpace(preparationOutput) != "" {
		fmt.Print(preparationOutput)
		if !strings.HasSuffix(preparationOutput, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("prepare Mattermost browser E2E: %w", errorValue)
	}
	preparation, errorValue := parseMattermostBrowserOpenE2EPreparation(preparationOutput)
	if errorValue != nil {
		return fmt.Errorf("parse Mattermost browser E2E preparation: %w", errorValue)
	}
	if !keep {
		defer func() {
			output, cleanupError := target.sshClient.runResult(cleanupMattermostBrowserOpenE2EScript(preparation))
			if strings.TrimSpace(output) != "" {
				fmt.Print(output)
				if !strings.HasSuffix(output, "\n") {
					fmt.Println()
				}
			}
			if cleanupError != nil {
				fmt.Fprintf(os.Stderr, "cleanup warning: %v\n", cleanupError)
			}
		}()
	}
	temporaryDirectory, errorValue := os.MkdirTemp("", "internkim-companion-browser-e2e-")
	if errorValue != nil {
		return errorValue
	}
	statePath := filepath.Join(temporaryDirectory, "state.json")
	browserProfilePath := filepath.Join(temporaryDirectory, "browser-profile")
	if keepBrowser {
		fmt.Println("local browser will remain open; temporary directory: " + temporaryDirectory)
	} else {
		defer os.RemoveAll(temporaryDirectory)
	}
	if errorValue := pairCompanionForMattermostBrowserOpenE2E(companionPath, preparation, statePath); errorValue != nil {
		return errorValue
	}
	closeMattermostBrowserOpenE2ESession(agentBrowserPath)
	if !keepBrowser {
		defer closeMattermostBrowserOpenE2ESession(agentBrowserPath)
	}
	companionCommand, companionLog, errorValue := startMattermostBrowserOpenE2ECompanion(companionPath, agentBrowserPath, statePath, browserProfilePath)
	if errorValue != nil {
		return errorValue
	}
	defer stopMattermostBrowserOpenE2ECompanion(companionCommand, companionLog)
	if errorValue := verifyMattermostBrowserOpenE2ECompanion(companionPath, agentBrowserPath, statePath); errorValue != nil {
		return errorValue
	}
	if errorValue := waitMattermostBrowserOpenE2ECompanionOnline(target, preparation, timeoutSeconds); errorValue != nil {
		return errorValue
	}
	if errorValue := target.runRemoteVerification(runMattermostBrowserOpenE2EScript(prompt, keep, timeoutSeconds, preparation)); errorValue != nil {
		return errorValue
	}
	return confirmMattermostBrowserOpenE2ELocalBrowser(agentBrowserPath)
}

func parseMattermostBrowserOpenE2EPreparation(output string) (mattermostBrowserOpenE2EPreparation, error) {
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var preparation mattermostBrowserOpenE2EPreparation
		if errorValue := json.Unmarshal([]byte(line), &preparation); errorValue == nil && preparation.Code != "" {
			return preparation, nil
		}
	}
	return mattermostBrowserOpenE2EPreparation{}, errors.New("preparation JSON was not found")
}

func closeMattermostBrowserOpenE2ESession(agentBrowserPath string) {
	command := exec.Command(agentBrowserPath, "--session", "internkim", "--session-name", "internkim", "close", "--all")
	_ = command.Run()
}

func pairCompanionForMattermostBrowserOpenE2E(companionPath string, preparation mattermostBrowserOpenE2EPreparation, statePath string) error {
	command := exec.Command(companionPath, "pair", "--device-url", preparation.DeviceURL, "--code", preparation.Code, "--state", statePath)
	output, errorValue := command.CombinedOutput()
	if strings.TrimSpace(string(output)) != "" {
		fmt.Print(string(output))
		if !strings.HasSuffix(string(output), "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("pair local companion: %w", errorValue)
	}
	return nil
}

func startMattermostBrowserOpenE2ECompanion(companionPath string, agentBrowserPath string, statePath string, browserProfilePath string) (*exec.Cmd, *bytes.Buffer, error) {
	command := exec.Command(
		companionPath,
		"run",
		"--state", statePath,
		"--agent-browser-path", agentBrowserPath,
		"--browser-profile", browserProfilePath,
		"--prefer-companion-browser",
		"--development-auto-approve-browser",
	)
	var logBuffer bytes.Buffer
	command.Stdout = &logBuffer
	command.Stderr = &logBuffer
	if errorValue := command.Start(); errorValue != nil {
		return nil, nil, fmt.Errorf("start local companion: %w", errorValue)
	}
	time.Sleep(3 * time.Second)
	if command.ProcessState != nil && command.ProcessState.Exited() {
		return nil, nil, fmt.Errorf("local companion exited early: %s", strings.TrimSpace(logBuffer.String()))
	}
	return command, &logBuffer, nil
}

func stopMattermostBrowserOpenE2ECompanion(command *exec.Cmd, logBuffer *bytes.Buffer) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Kill()
	_, _ = command.Process.Wait()
	if strings.TrimSpace(logBuffer.String()) != "" {
		fmt.Print(logBuffer.String())
		if !strings.HasSuffix(logBuffer.String(), "\n") {
			fmt.Println()
		}
	}
}

func verifyMattermostBrowserOpenE2ECompanion(companionPath string, agentBrowserPath string, statePath string) error {
	command := exec.Command(companionPath, "status", "--state", statePath, "--json", "--verify-auth")
	command.Env = append(os.Environ(), "INTERNKIM_AGENT_BROWSER_PATH="+agentBrowserPath)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("verify local companion status: %w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	var statusDocument struct {
		AuthStatus           string `json:"authStatus"`
		BrowserRuntimeStatus string `json:"browserRuntimeStatus"`
	}
	if errorValue := json.Unmarshal(output, &statusDocument); errorValue != nil {
		return fmt.Errorf("parse local companion status: %w", errorValue)
	}
	if statusDocument.AuthStatus != "verified" {
		return fmt.Errorf("local companion auth status is %s", statusDocument.AuthStatus)
	}
	if statusDocument.BrowserRuntimeStatus != "ready" {
		return fmt.Errorf("local companion browser runtime status is %s", statusDocument.BrowserRuntimeStatus)
	}
	fmt.Println("local companion: verified, browser runtime ready")
	return nil
}

func waitMattermostBrowserOpenE2ECompanionOnline(target verifyTarget, preparation mattermostBrowserOpenE2EPreparation, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	output, errorValue := target.sshClient.runResult(waitMattermostBrowserOpenE2ECompanionOnlineScript(preparation, timeoutSeconds))
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("wait for remote companion heartbeat: %w", errorValue)
	}
	return nil
}

func confirmMattermostBrowserOpenE2ELocalBrowser(agentBrowserPath string) error {
	command := exec.Command(agentBrowserPath, "--session", "internkim", "--session-name", "internkim", "get", "url")
	output, errorValue := command.CombinedOutput()
	localURL := strings.TrimSpace(string(output))
	if errorValue != nil {
		return fmt.Errorf("confirm local browser URL: %w: %s", errorValue, localURL)
	}
	if !strings.Contains(localURL, "google.") {
		return fmt.Errorf("local browser did not navigate to Google: %s", localURL)
	}
	fmt.Println("local browser URL: " + localURL)
	return nil
}

func runVerifyBrowser(arguments []string) error {
	flagSet := flag.NewFlagSet("verify browser", flag.ContinueOnError)
	localMode := flagSet.Bool("local", false, "Run local Mattermost browser smoke test")
	publicMode := flagSet.Bool("public", false, "Run public URL browser smoke test")
	target := registerTargetFlags(flagSet)
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if !*localMode && !*publicMode {
		*localMode = true
	}

	verifyTarget, errorValue := target.resolveVerifyTarget()
	if errorValue != nil {
		return errorValue
	}
	if *localMode {
		if errorValue := runLocalBrowserVerification(verifyTarget); errorValue != nil {
			return errorValue
		}
	}
	if *publicMode {
		if errorValue := runPublicBrowserVerification(verifyTarget); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func resolveVerifyTarget(arguments []string) (verifyTarget, error) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return verifyTarget{}, errorValue
	}

	flagSet := flag.NewFlagSet("verify", flag.ContinueOnError)
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	flagSet.String("node", "", "Fleet node target")
	flagSet.Bool("cloudflare-ssh", false, "Use Cloudflare SSH")
	flagSet.String("board", "", "Board target")
	flagSet.Bool("sim", false, "Use simulation target")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return verifyTarget{}, errorValue
	}

	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	target := resolveCommandTarget(arguments)
	if strings.TrimSpace(*host) != "" {
		target.host = strings.TrimSpace(*host)
		if strings.TrimSpace(*user) != "" {
			target.sshUser = strings.TrimSpace(*user)
		}
		if *password != "" {
			target.sshPassword = *password
		}
	}
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	target = simulationHostTarget(repositoryRootPath, target)
	configuration := loadConfig()
	if target.useRemoteSSH && strings.TrimSpace(target.host) == "" {
		target.host = firstNonEmptyString(target.sshHostname, resolveCloudflareSSHHostname(configuration, target))
	}
	if !target.useRemoteSSH && strings.TrimSpace(target.host) == "" && target.mode != commandTargetModeSimulation {
		target.host = findBoardIPForCredentials(sshpassBin, target.stateDir, target.sshUser, target.sshPassword)
	}
	sshClient := (*sshClient)(nil)
	if strings.TrimSpace(target.host) != "" {
		sshClient = newVerifySSHClient(sshpassBin, target)
	} else {
		connection, isRemote, connectionError := resolveDeviceSSHConnection(configuration, sshpassBin, target)
		if connectionError != nil || connection == nil {
			return verifyTarget{}, errors.New("verify target not found; pass --host <ip>")
		}
		sshClient = connection
		target.host = connection.host
		target.useRemoteSSH = isRemote
	}

	printCommandTargetEvidence(target)
	return verifyTarget{
		host:       target.host,
		user:       target.sshUser,
		password:   target.sshPassword,
		nodeID:     target.nodeID,
		scriptDir:  repositoryRootPath,
		stateDir:   target.stateDir,
		sshpassBin: sshpassBin,
		sshClient:  sshClient,
	}, nil
}

func newVerifySSHClient(sshpassBin string, target commandTarget) *sshClient {
	if target.useRemoteSSH {
		return newCloudflareSSH(sshpassBin, target.sshUser, target.sshPassword, target.host)
	}
	return newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host)
}

func (target verifyTarget) runRemoteVerification(script string) error {
	return target.runRemoteVerificationWithTimeout(script, 60*time.Second)
}

func (target verifyTarget) runRemoteVerificationWithTimeout(script string, timeout time.Duration) error {
	output, errorValue := target.sshClient.runResultWithTimeout(script, timeout)
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("remote verification failed: %w", errorValue)
	}
	return nil
}

func (target verifyTarget) runMattermostPromptVerification(script string, timeout time.Duration, downloadDirectory string) error {
	output, errorValue := target.sshClient.runResultWithTimeout(script, timeout)
	if strings.TrimSpace(output) != "" {
		fmt.Print(redactDownloadedMattermostFiles(output))
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("remote verification failed: %w", errorValue)
	}
	if strings.TrimSpace(downloadDirectory) == "" {
		return nil
	}
	return writeDownloadedMattermostFiles(output, downloadDirectory)
}

type downloadedMattermostFile struct {
	FileID        string `json:"fileID"`
	Filename      string `json:"filename"`
	ContentType   string `json:"contentType"`
	ContentBase64 string `json:"contentBase64"`
}

type mattermostVerificationOutput struct {
	DownloadedFiles           []downloadedMattermostFile `json:"downloadedFiles"`
	SiteScreenshots           []downloadedMattermostFile `json:"siteScreenshots"`
	BotMessage                string                     `json:"botMessage"`
	FileIDs                   []string                   `json:"fileIDs"`
	TaskRunID                 string                     `json:"taskRunID"`
	TaskStatus                *string                    `json:"taskStatus"`
	SitePublicURL             string                     `json:"sitePublicURL"`
	SiteHTMLText              string                     `json:"siteHTMLText"`
	SiteHTMLRaw               string                     `json:"siteHTMLRaw"`
	SiteCSSRaw                string                     `json:"siteCSSRaw"`
	SiteStyleMetrics          map[string]any             `json:"siteStyleMetrics"`
	SiteScreenshotFiles       []string                   `json:"siteScreenshotFiles"`
	IsSiteScreenshotsVerified bool                       `json:"siteScreenshotsVerified"`
	IsAutoConfirmationSent    bool                       `json:"autoConfirmationSent"`
	IsSuccessful              bool                       `json:"ok"`
	FailureReason             string                     `json:"failureReason"`
}

func parseMattermostVerificationOutput(output string) (mattermostVerificationOutput, error) {
	document, found := parseLastJSONDocument(output)
	if !found {
		return mattermostVerificationOutput{}, fmt.Errorf("remote verification did not return JSON output")
	}
	var verificationOutput mattermostVerificationOutput
	if errorValue := json.Unmarshal(document, &verificationOutput); errorValue != nil {
		return mattermostVerificationOutput{}, fmt.Errorf("parse remote verification JSON: %w", errorValue)
	}
	return verificationOutput, nil
}

func writeDownloadedMattermostFiles(output string, downloadDirectory string) error {
	_, errorValue := writeDownloadedMattermostFilesWithOption(output, downloadDirectory, false)
	return errorValue
}

func writeDownloadedMattermostFilesAllowEmpty(output string, downloadDirectory string) ([]string, error) {
	return writeDownloadedMattermostFilesWithOption(output, downloadDirectory, true)
}

func writeDownloadedMattermostFilesWithOption(output string, downloadDirectory string, canBeEmpty bool) ([]string, error) {
	verificationOutput, errorValue := parseMattermostVerificationOutput(output)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(verificationOutput.DownloadedFiles) == 0 {
		if canBeEmpty {
			return nil, nil
		}
		return nil, fmt.Errorf("remote verification returned no downloaded Mattermost attachments")
	}
	if errorValue := os.MkdirAll(downloadDirectory, 0o755); errorValue != nil {
		return nil, fmt.Errorf("create download directory: %w", errorValue)
	}
	downloadedFilePaths := []string{}
	for _, downloadedFile := range verificationOutput.DownloadedFiles {
		downloadedFilePath, errorValue := writeDownloadedMattermostFile(downloadedFile, downloadDirectory)
		if errorValue != nil {
			return nil, errorValue
		}
		downloadedFilePaths = append(downloadedFilePaths, downloadedFilePath)
	}
	return downloadedFilePaths, nil
}

func writeDownloadedMattermostFile(downloadedFile downloadedMattermostFile, downloadDirectory string) (string, error) {
	filename := safeDownloadedMattermostFilename(downloadedFile)
	return writeDownloadedMattermostFileToPath(downloadedFile, filepath.Join(downloadDirectory, filename))
}

func writeDownloadedMattermostFileToPath(downloadedFile downloadedMattermostFile, outputPath string) (string, error) {
	content, errorValue := base64.StdEncoding.DecodeString(downloadedFile.ContentBase64)
	if errorValue != nil {
		return "", fmt.Errorf("decode Mattermost attachment %s: %w", downloadedFile.FileID, errorValue)
	}
	parentPath := filepath.Dir(outputPath)
	if parentPath != "." {
		if errorValue := os.MkdirAll(parentPath, 0o755); errorValue != nil {
			return "", fmt.Errorf("create Mattermost attachment parent directory: %w", errorValue)
		}
	}
	if errorValue := os.WriteFile(outputPath, content, 0o644); errorValue != nil {
		return "", fmt.Errorf("write Mattermost attachment %s: %w", outputPath, errorValue)
	}
	fmt.Println("downloaded Mattermost attachment: " + outputPath)
	return outputPath, nil
}

func safeDownloadedMattermostFilename(downloadedFile downloadedMattermostFile) string {
	filename := strings.TrimSpace(filepath.Base(downloadedFile.Filename))
	if filename != "" && filename != "." {
		return filename
	}
	fileID := strings.TrimSpace(downloadedFile.FileID)
	if fileID != "" {
		return fileID
	}
	return "mattermost-attachment"
}

func redactDownloadedMattermostFiles(output string) string {
	document, found := parseLastJSONDocument(output)
	if !found {
		return output
	}
	var payload map[string]any
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		return output
	}
	redactBase64Attachments(payload, "downloadedFiles")
	redactBase64Attachments(payload, "siteScreenshots")
	redactedDocument, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return output
	}
	return replaceLastJSONDocument(output, string(redactedDocument))
}

func redactBase64Attachments(payload map[string]any, fieldName string) {
	files, isArray := payload[fieldName].([]any)
	if !isArray {
		return
	}
	for _, value := range files {
		fileDocument, isDocument := value.(map[string]any)
		if !isDocument {
			continue
		}
		contentBase64, isString := fileDocument["contentBase64"].(string)
		if isString && contentBase64 != "" {
			fileDocument["contentBase64"] = fmt.Sprintf("<redacted %d base64 chars>", len(contentBase64))
		}
	}
}

func parseLastJSONDocument(output string) ([]byte, bool) {
	trimmedOutput := strings.TrimSpace(output)
	for index := len(trimmedOutput) - 1; index >= 0; index-- {
		if trimmedOutput[index] != '{' {
			continue
		}
		if index > 0 && trimmedOutput[index-1] != '\n' && trimmedOutput[index-1] != '\r' {
			continue
		}
		candidate := trimmedOutput[index:]
		var document map[string]any
		decoder := json.NewDecoder(strings.NewReader(candidate))
		if decoder.Decode(&document) == nil {
			return []byte(candidate[:int(decoder.InputOffset())]), true
		}
	}
	return nil, false
}

func replaceLastJSONDocument(output string, replacement string) string {
	trimmedOutput := strings.TrimSpace(output)
	document, found := parseLastJSONDocument(trimmedOutput)
	if !found {
		return output
	}
	index := strings.LastIndex(trimmedOutput, string(document))
	if index < 0 {
		return output
	}
	prefix := trimmedOutput[:index]
	if prefix == "" {
		return replacement + "\n"
	}
	return prefix + replacement + "\n"
}

func mattermostPromptSSHTimeout(timeoutSeconds int) time.Duration {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	return time.Duration(timeoutSeconds+180) * time.Second
}

func mattermostPromptScriptSSHTimeout(timeoutSeconds int, expectPublicURL bool) time.Duration {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	total := 120 + timeoutSeconds + 60 + timeoutSeconds
	if expectPublicURL {
		total += timeoutSeconds + 135
	}
	return time.Duration(total+180) * time.Second
}


func runLocalBrowserVerification(target verifyTarget) error {
	port, errorValue := reserveLocalPort()
	if errorValue != nil {
		return errorValue
	}

	tunnelCommand := buildMattermostTunnelCommand(target, port)
	tunnelCommand.Stdout = os.Stdout
	tunnelCommand.Stderr = os.Stderr
	if errorValue := tunnelCommand.Start(); errorValue != nil {
		return errorValue
	}
	defer func() {
		_ = tunnelCommand.Process.Kill()
		_, _ = tunnelCommand.Process.Wait()
	}()
	time.Sleep(1500 * time.Millisecond)

	adminEmail := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/config/admin-email 2>/dev/null || cat /root/.internkim/admin-email 2>/dev/null"))
	adminPassword := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/secrets/mm-admin-pass 2>/dev/null"))
	return runPlaywright("tests/e2e/mattermost.spec.ts", map[string]string{
		"INTERNKIM_MATTERMOST_URL": fmt.Sprintf("http://127.0.0.1:%d", port),
		"INTERNKIM_ADMIN_EMAIL":    adminEmail,
		"INTERNKIM_ADMIN_PASSWORD": adminPassword,
	})
}

func runPublicBrowserVerification(target verifyTarget) error {
	publicURL := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/env/mattermost-url 2>/dev/null"))
	if publicURL == "" {
		return errors.New("public Mattermost URL is empty")
	}
	return runPlaywright("tests/e2e/public-url.spec.ts", map[string]string{
		"INTERNKIM_PUBLIC_URL": publicURL,
	})
}

func reserveLocalPort() (int, error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, errorValue
	}
	defer listener.Close()
	address, isTCPAddress := listener.Addr().(*net.TCPAddr)
	if !isTCPAddress {
		return 0, errors.New("failed to reserve local tcp port")
	}
	return address.Port, nil
}

func buildMattermostTunnelCommand(target verifyTarget, port int) *exec.Cmd {
	sshArguments := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ExitOnForwardFailure=yes",
		"-N",
		"-L", fmt.Sprintf("%d:127.0.0.1:8065", port),
		fmt.Sprintf("%s@%s", target.user, target.host),
	}
	if target.password != "" {
		return exec.Command(target.sshpassBin, append([]string{"-p", target.password, "ssh"}, sshArguments...)...)
	}
	return exec.Command("ssh", sshArguments...)
}

func runPlaywright(specPath string, environmentVariables map[string]string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	command := exec.Command("bunx", "playwright", "test", specPath)
	command.Dir = filepath.Join(repositoryRootPath, "web")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	for key, value := range environmentVariables {
		command.Env = append(command.Env, key+"="+value)
	}
	return command.Run()
}

func verifyAPIScript() string {
	script := `set -euo pipefail

echo "checking services"
systemctl is-active mattermost | grep -q '^active$'
systemctl is-active blueclaw | grep -q '^active$'
systemctl is-active internkim-admind | grep -q '^active$'
if systemctl cat cloudflared >/dev/null 2>&1; then
  systemctl is-active cloudflared | grep -q '^active$'
fi
grep -q 'blueclaw-supervisor' /etc/systemd/system/blueclaw.service
! grep -q 'ExecStart=/usr/local/bin/blueclaw ' /etc/systemd/system/blueclaw.service
test -x /usr/local/bin/firecracker
test -x /usr/local/bin/jailer
test -s /opt/internkim/blueclaw-runtime/manifest.json
test -s /opt/internkim/blueclaw-runtime/payload-manifest.json
test -s /opt/internkim/blueclaw-runtime/vmlinux.bin
test -s /opt/internkim/blueclaw-runtime/rootfs.ext4
blkid -o value -s TYPE /var/lib/blueclaw/workspace.ext4 | grep -q '^ext4$'
` + browserruntime.DeviceReadinessShellScript() + `

echo "checking admin gateway"
curl --silent --show-error --fail http://127.0.0.1:18080/admin/api/health | jq -e '.status == "ok"' >/dev/null
curl --silent --show-error --fail http://127.0.0.1:18080/admin/ | grep -q '<script'
curl --silent --show-error --fail http://127.0.0.1:18080/admin/_app/version.json | jq -e '.version | length > 0' >/dev/null

echo "checking mattermost ping"
curl --silent --show-error --fail http://localhost:8065/api/v4/system/ping | jq -e '.status == "OK"' >/dev/null

echo "checking capabilityd health"
curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock http://internkim/health | jq -e '.status == "ok"' >/dev/null

echo "checking admin login"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_headers="$(mktemp)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

echo "checking capability profile lookup"
mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $mattermost_token" http://localhost:8065/api/v4/users/me | jq -r '.id // empty')"
test -n "$bot_user_id"
lookup_body="$(jq -cn --arg senderID "$bot_user_id" '{senderID:$senderID}')"
curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$lookup_body" http://internkim/v1/platform/mattermost/identity.resolve | jq -e '.email != null' >/dev/null

echo "checking llm capability"
model="$(jq -r '.languageModel.capability.model // "__DEFAULT_OPENROUTER_MODEL__"' /root/.blueclaw/config/runtime.json)"
llm_text_body="$(jq -cn --arg model "$model" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Reply with ok."}],
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_text_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_text_body" http://internkim/v1/llm/text)"
printf '%s' "$llm_text_response" | jq -e '.content | type == "string" and length > 0' >/dev/null
schema='{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}'
llm_structured_body="$(jq -cn --arg model "$model" --argjson schema "$schema" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Return JSON only with reply set to ok."}],
  structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_structured_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_structured_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_structured_response" | jq -e '.content | fromjson | .reply | type == "string"' >/dev/null
llm_auto_structured_body="$(jq -cn --arg model "$model" --argjson schema "$schema" '{
  model: $model,
  executionMode: "auto",
  messages: [{role:"user", content:"Return JSON only with reply set to ok."}],
  structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_auto_structured_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_auto_structured_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_auto_structured_response" | jq -e '.provider == "openrouter" and .selectedBackend == "remote" and (.content | fromjson | .reply | type == "string")' >/dev/null
action_schema='{"oneOf":[{"type":"object","properties":{"action":{"type":"string","enum":["finish"]},"message":{"type":"string"},"goalStatus":{"type":"string","enum":["satisfied"]},"goalSatisfied":{"type":"boolean"},"completionEvidence":{"type":"array","items":{"type":"object"}},"qualityReview":{"type":"array","items":{"type":"object"}}},"required":["action","message","goalStatus","goalSatisfied","completionEvidence","qualityReview"],"additionalProperties":false}]}'
llm_action_body="$(jq -cn --arg model "$model" --argjson schema "$action_schema" '{
  model: $model,
  executionMode: "auto",
  messages: [
    {role:"system", content:"You must finish this smoke test now. Use the finish action with message ok, goalStatus satisfied, goalSatisfied true, and empty evidence/review arrays."},
    {role:"user", content:"Finish now."}
  ],
  structuredOutputSchema: {name:"blueclaw_agent_turn_action", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_action_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_action_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_action_response" | jq -e '.provider == "openrouter" and .selectedBackend == "remote" and .constraintMode == "native_tool_call" and (.content | fromjson | .action == "finish")' >/dev/null

echo "checking litert capability"
if command -v litert-lm >/dev/null 2>&1 && [ -s /root/.internkim/models/gemma-4-E4B-it.litertlm ]; then
  litert_body="$(jq -cn '{
    model: "local/gemma-4-E4B-it-litert-lm",
    accelerator: "cpu",
    executionMode: "device",
    messages: [{role:"user", content:"Reply with ok."}],
    requireParameters: true,
    enableResponseHealing: true
  }')"
  litert_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$litert_body" http://internkim/v1/llm/text 2>/tmp/internkim-verify-litert-error || true)"
  if ! printf '%s' "$litert_response" | python3 -c 'import json, sys; document=json.load(sys.stdin); backend=document.get("selectedBackend"); content=document.get("content"); raise SystemExit(0 if backend in ("gpu", "cpu") and isinstance(content, str) and len(content) > 0 else 1)' >/dev/null 2>&1; then
    if [ -n "$litert_response" ]; then
      printf 'litert capability: %s\n' "$(printf '%s' "$litert_response" | tr '\n' ' ' | cut -c1-180)"
    else
      printf 'litert capability: %s\n' "$(tr '\n' ' ' </tmp/internkim-verify-litert-error | cut -c1-180)"
    fi
    echo "litert capability: optional local check failed"
  fi
else
  echo "litert capability: skipped"
fi

echo "checking secret isolation"
! su -s /bin/sh blueclaw -c 'test -r /root/.internkim/secrets/openrouter-api-key || test -r /root/.internkim/secrets/mattermost-bot-token || test -r /root/.internkim/secrets/slack-bot-token || test -r /root/.internkim/secrets/slack-app-token || test -r /root/.internkim/secrets/device-secret || test -r /root/.internkim/config/signal-jsonrpc-url || test -r /root/.internkim/config/signal-account || test -r /root/.internkim/models/gemma-4-E4B-it.litertlm' 2>/dev/null

echo "checking blueclaw health"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/health | jq -e '.status == "ok"' >/dev/null

echo "checking blueclaw backup manifest"
manifest_path="$(mktemp)"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/backup/manifest > "$manifest_path"
python3 - "$manifest_path" <<'PY'
import json
import sys

with open(sys.argv[1]) as file:
    manifest = json.load(file)

document = json.dumps(manifest)
for forbidden in ("token", "Authorization", "passphrase", "OpenRouter", "apiKey", "signingSecret"):
    if forbidden in document:
        print("backup manifest contains secret reference: " + forbidden, file=sys.stderr)
        sys.exit(1)
if manifest.get("contractVersion") != 1:
    print("unexpected backup contract version", file=sys.stderr)
    sys.exit(1)
if "blueclaw-postgres-dump" not in manifest.get("requiredBackupArtifacts", []):
    print("missing blueclaw postgres backup artifact", file=sys.stderr)
    sys.exit(1)
PY

echo "checking users sync"
systemctl start internkim-users-sync.service || journalctl -u internkim-users-sync -n 40 --no-pager
test -f /root/.internkim/state/users-sync.json
policy_response_path="$(mktemp)"
trap 'rm -f "$policy_response_path"' EXIT
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/policy > "$policy_response_path"
python3 - "$policy_response_path" <<'PY'
import json
import sys

with open("/root/.internkim/state/users-sync.json") as file:
    expected = {email.lower() for email in json.load(file).get("users", [])}
with open(sys.argv[1]) as file:
    policy = json.load(file)
actual = set()
for person in policy.get("people", []):
    for email in person.get("emails", []):
        actual.add(str(email).lower())
missing = sorted(expected - actual)
if missing:
    print("missing policy emails: " + ", ".join(missing), file=sys.stderr)
    sys.exit(1)
PY

echo "verify api: ok"
`
	return strings.ReplaceAll(script, "__DEFAULT_OPENROUTER_MODEL__", blueclaw.BlueclawDefaultModelName)
}

func verifyMattermostScript() string {
	return `set -euo pipefail

timestamp="$(date +%s)"
invited_email="verify-invited-$timestamp@internkim.test"
uninvited_email="verify-uninvited-$timestamp@internkim.test"
invited_username="verifyinvited$timestamp"
uninvited_username="verifyuninvited$timestamp"
password="VerifyPass!$timestamp-InternKim-Mattermost"
team_name="internkim"
verify_channel_name="verify-$timestamp"
verify_channel_display_name="Verify $timestamp"
channel_id=""
team_id=""
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
test_started_at="$(date +%s%3N)"

phase() {
  echo "$1"
}

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    fi
  else
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" "$url")" || curl_status="$?"
    fi
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

resolve_team_id() {
  api_request "resolve team" GET "http://localhost:8065/api/v4/teams/name/$team_name" "$admin_token" | jq -r '.id // empty'
}

create_verify_channel() {
  local body
  body="$(jq -cn --arg team_id "$team_id" --arg name "$verify_channel_name" --arg display_name "$verify_channel_display_name" '{team_id:$team_id,name:$name,display_name:$display_name,type:"P"}')"
  api_request "create verify channel" POST http://localhost:8065/api/v4/channels "$admin_token" "$body" | jq -r '.id // empty'
}

phase "admin login"
login_headers="$(mktemp)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

create_user() {
  local email="$1"
  local username="$2"
  local body
  body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
  api_request "create user $username" POST http://localhost:8065/api/v4/users "$admin_token" "$body" | jq -r '.id'
}

login_user() {
  local username="$1"
  local headers
  headers="$(mktemp)"
  local body
  body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
  curl --silent --show-error --fail -D "$headers" -o /tmp/internkim-user-login.json \
    -H "Content-Type: application/json" \
    -d "$body" \
    http://localhost:8065/api/v4/users/login >/dev/null
  awk 'tolower($1) == "token:" {print $2}' "$headers" | tr -d '\r'
}

join_channel() {
  local user_id="$1"
  local team_member_body
  local channel_member_body
  if [ -z "$team_id" ]; then
    team_id="$(resolve_team_id)"
  fi
  if [ -z "$team_id" ]; then
    echo "Mattermost team $team_name was not found" >&2
    return 1
  fi
  if [ -z "$channel_id" ]; then
    echo "Mattermost verify channel was not created" >&2
    return 1
  fi
  team_member_body="$(jq -cn --arg team_id "$team_id" --arg user_id "$user_id" '{team_id:$team_id,user_id:$user_id}')"
  api_request "join team $user_id" POST "http://localhost:8065/api/v4/teams/$team_id/members" "$admin_token" "$team_member_body" >/dev/null
  channel_member_body="$(jq -cn --arg user_id "$user_id" '{user_id:$user_id}')"
  api_request "join channel $user_id" POST "http://localhost:8065/api/v4/channels/$channel_id/members" "$admin_token" "$channel_member_body" >/dev/null
  api_request "verify channel membership $user_id" GET "http://localhost:8065/api/v4/channels/$channel_id/members/$user_id" "$admin_token" >/dev/null
}

post_message() {
  local user_token="$1"
  local message="$2"
  api_request "post message" POST http://localhost:8065/api/v4/posts "$user_token" \
    "$(jq -cn --arg channel_id "$channel_id" --arg message "$message" '{channel_id:$channel_id,message:$message}')"
}

task_count() {
  blueclaw_request "task count" GET http://127.0.0.1:8080/admin/api/task | jq 'length'
}

wait_for_task_count() {
  local expected_count="$1"
  for _ in $(seq 1 120); do
    local current_count
    current_count="$(task_count)"
    if [ "$current_count" -ge "$expected_count" ]; then
      return 0
    fi
    sleep 1
  done
  echo "expected task count >= $expected_count" >&2
  return 1
}

print_recent_bot_replies() {
  local posted_after="$1"
  api_request "print recent bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$posted_after" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | "\(.create_at)\t\(.message)"' >&2 || true
}

wait_for_bot_reply() {
  local expected_text="$1"
  local posted_after="$2"
  for _ in $(seq 1 120); do
    if api_request "wait for bot reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=30" "$admin_token" |
      jq -e --arg bot_user_id "$bot_user_id" --arg expected_text "$expected_text" --argjson posted_after "$posted_after" \
        '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after and (.message | contains($expected_text)))' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "expected bot reply containing: $expected_text" >&2
  print_recent_bot_replies "$posted_after"
  return 1
}

wait_for_model_reply() {
  local posted_after="$1"
  for _ in $(seq 1 120); do
    if api_request "wait for model reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
      jq -e --arg bot_user_id "$bot_user_id" --argjson posted_after "$posted_after" \
        '.posts[] | select(
          .user_id == $bot_user_id and
          .create_at >= $posted_after and
          (.message | contains("I am having trouble reaching the language model") | not) and
          (.message | contains("has not invited") | not)
        )' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "expected model-generated bot reply after post timestamp: $posted_after" >&2
  print_recent_bot_replies "$posted_after"
  return 1
}

delete_post() {
  local token="$1"
  local post_id="$2"
  if [ -z "$post_id" ] || [ "$post_id" = "null" ]; then
    return 0
  fi
  if [ -z "$token" ]; then
    echo "cleanup warning: missing token for Mattermost post $post_id" >&2
    return 0
  fi
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $token" \
    "http://localhost:8065/api/v4/posts/$post_id" >/dev/null || echo "cleanup warning: failed to delete Mattermost post $post_id" >&2
}

delete_user() {
  local user_id="$1"
  if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
    return 0
  fi
  if [ -n "${channel_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/channels/$channel_id/members/$user_id" >/dev/null || true
  fi
  if [ -z "${team_id:-}" ]; then
    team_id="$(resolve_team_id 2>/dev/null || true)"
  fi
  if [ -n "$team_id" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/teams/$team_id/members/$user_id" >/dev/null || true
  fi
  curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id" >/dev/null || \
    echo "cleanup warning: failed to delete Mattermost user $user_id" >&2
}

delete_stale_verify_users() {
  for username_prefix in verifyinvited verifyuninvited; do
    api_request "find stale users $username_prefix" POST http://localhost:8065/api/v4/users/search "$admin_token" \
      "$(jq -cn --arg term "$username_prefix" '{term:$term}')" |
      jq -r --arg username_prefix "$username_prefix" '.[] | select(.username | startswith($username_prefix)) | .id' |
      while read -r user_id; do
        delete_user "$user_id"
      done
  done
}

delete_verify_system_posts() {
  if [ "$(id -u)" != "0" ]; then
    echo "cleanup warning: skipping verify Mattermost system posts; sudo is unavailable" >&2
    return 0
  fi
  su -s /bin/sh postgres -c "psql -d mattermost" <<'SQL' >/dev/null || echo "cleanup warning: failed to delete verify Mattermost system posts" >&2
UPDATE posts
SET deleteat = (extract(epoch from now()) * 1000)::bigint
WHERE type LIKE 'system_%'
  AND deleteat = 0
  AND message ~ '(verifyinvited|verifyuninvited|labmattermost)';
SQL
}

delete_verify_replies() {
  local token="$1"
  if [ -z "${channel_id:-}" ]; then
    return 0
  fi
  api_request "cleanup enumerate bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=100" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson test_started_at "$test_started_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $test_started_at) | .id' |
    while read -r post_id; do
      delete_post "$token" "$post_id"
    done || echo "cleanup warning: failed to enumerate Mattermost bot replies" >&2
}

delete_verify_channel() {
  if [ -z "${channel_id:-}" ]; then
    return 0
  fi
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/channels/$channel_id" >/dev/null || echo "cleanup warning: failed to delete Mattermost verify channel $channel_id" >&2
}

cleanup() {
  delete_post "${invited_token:-}" "${invited_post_id:-}"
  delete_post "${uninvited_token:-}" "${uninvited_post_id:-}"
  delete_verify_replies "${mattermost_token:-}"
  delete_verify_system_posts
  delete_user "${invited_user_id:-}"
  delete_user "${uninvited_user_id:-}"
  delete_verify_channel
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$invited_email" >/dev/null || true
}
trap cleanup EXIT

phase "cleanup stale verify users"
delete_verify_system_posts
delete_stale_verify_users
phase "bot lookup"
mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_profile="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token")"
bot_user_id="$(printf '%s' "$bot_profile" | jq -r '.id // empty')"
bot_username="$(printf '%s' "$bot_profile" | jq -r '.username // empty')"
test -n "$bot_user_id"
test -n "$bot_username"
phase "create verify channel"
team_id="$(resolve_team_id)"
test -n "$team_id"
channel_id="$(create_verify_channel)"
test -n "$channel_id"
join_channel "$bot_user_id"
phase "create users"
invited_user_id="$(create_user "$invited_email" "$invited_username")"
uninvited_user_id="$(create_user "$uninvited_email" "$uninvited_username")"
phase "join users"
join_channel "$invited_user_id"
join_channel "$uninvited_user_id"
phase "login users"
invited_token="$(login_user "$invited_username")"
uninvited_token="$(login_user "$uninvited_username")"
test -n "$invited_token"
test -n "$uninvited_token"

phase "invite policy"
blueclaw_request "invite policy" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$invited_user_id" --arg email "$invited_email" '{personID:$personID,email:$email}')" >/dev/null

phase "invited post"
before_count="$(task_count)"
invited_message="InternKim Mattermost verification $timestamp: please reply briefly."
invited_post="$(post_message "$invited_token" "$invited_message")"
invited_post_id="$(printf '%s' "$invited_post" | jq -r '.id')"
invited_post_create_at="$(printf '%s' "$invited_post" | jq -r '.create_at')"
test -n "$invited_post_id"
test -n "$invited_post_create_at"
phase "reply wait"
wait_for_task_count "$((before_count + 1))"
wait_for_model_reply "$invited_post_create_at"
after_count="$(task_count)"

phase "uninvited post"
uninvited_message="@$bot_username InternKim uninvited Mattermost verification $timestamp: please reply briefly."
uninvited_post="$(post_message "$uninvited_token" "$uninvited_message")"
uninvited_post_id="$(printf '%s' "$uninvited_post" | jq -r '.id')"
uninvited_post_create_at="$(printf '%s' "$uninvited_post" | jq -r '.create_at')"
test -n "$uninvited_post_id"
test -n "$uninvited_post_create_at"
phase "rejection wait"
wait_for_bot_reply "has not invited" "$uninvited_post_create_at"
final_count="$(task_count)"
test "$final_count" -eq "$after_count"

echo "verify mattermost: ok"
`
}

func verifyMattermostPromptScript(prompt string, keep bool, timeoutSeconds int, expectBrowserOpen bool, expectPublicURL bool, expectedTools []string, expectedEvents []string, downloadFiles bool, waitForCompletion bool, autoConfirm bool) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	encodedPrompt := base64.StdEncoding.EncodeToString([]byte(prompt))
	expectedToolsJSON, _ := json.Marshal(trimmedNonEmptyValues(expectedTools))
	expectedEventsJSON, _ := json.Marshal(trimmedNonEmptyValues(expectedEvents))
	encodedExpectedTools := base64.StdEncoding.EncodeToString(expectedToolsJSON)
	encodedExpectedEvents := base64.StdEncoding.EncodeToString(expectedEventsJSON)
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	expectBrowserOpenValue := "false"
	if expectBrowserOpen {
		expectBrowserOpenValue = "true"
	}
	expectPublicURLValue := "false"
	if expectPublicURL {
		expectPublicURLValue = "true"
	}
	downloadFilesValue := "false"
	if downloadFiles {
		downloadFilesValue = "true"
	}
	waitForCompletionValue := "false"
	if waitForCompletion {
		waitForCompletionValue = "true"
	}
	autoConfirmValue := "false"
	if autoConfirm {
		autoConfirmValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

timestamp="$(date +%%s)"
email="probe-mattermost-$timestamp@internkim.test"
username="probemm$timestamp"
password="ProbePass!$timestamp-InternKim-Mattermost"
prompt="$(printf '%%s' %s | base64 -d)"
expected_tools_json="$(printf '%%s' %s | base64 -d)"
expected_events_json="$(printf '%%s' %s | base64 -d)"
keep_artifacts=%s
timeout_seconds=%d
expect_browser_open=%s
expect_public_url=%s
download_files=%s
wait_for_completion=%s
auto_confirm=%s
test_started_at="$(date +%%s%%3N)"

health_timeout_seconds=120
reply_timeout_seconds=$timeout_seconds
registration_timeout_seconds=60
completion_timeout_seconds=$timeout_seconds
public_url_timeout_seconds=$timeout_seconds

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    fi
  else
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" "$url")" || curl_status="$?"
    fi
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

download_bot_files() {
  if [ "$download_files" != "true" ]; then
    return 0
  fi
  printf '[]' > "$downloaded_files_file"
  for file_id in $(jq -r '.file_ids[]?' "$bot_post_file"); do
    file_info="$(api_request "fetch bot attachment info" GET "http://localhost:8065/api/v4/files/$file_id/info" "$admin_token")"
    filename="$(printf '%%s' "$file_info" | jq -r --arg file_id "$file_id" '.name // .filename // $file_id')"
    content_type="$(printf '%%s' "$file_info" | jq -r '.mime_type // .content_type // ""')"
    attachment_file="$(mktemp)"
    content_base64_file="$(mktemp)"
    curl --silent --show-error --fail \
      -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/files/$file_id" \
      -o "$attachment_file"
    base64 -w 0 "$attachment_file" > "$content_base64_file" 2>/dev/null || base64 "$attachment_file" | tr -d '\n' > "$content_base64_file"
    next_downloaded_files_file="$(mktemp)"
    jq \
      --arg file_id "$file_id" \
      --arg filename "$filename" \
      --arg content_type "$content_type" \
      --rawfile content_base64 "$content_base64_file" \
      '. + [{fileID:$file_id, filename:$filename, contentType:$content_type, contentBase64:$content_base64}]' \
      "$downloaded_files_file" > "$next_downloaded_files_file"
    mv "$next_downloaded_files_file" "$downloaded_files_file"
    rm -f "$attachment_file" "$content_base64_file"
  done
}

wait_for_blueclaw_health() {
  for _ in $(seq 1 "$health_timeout_seconds"); do
    if curl --silent --show-error --fail --max-time 15 http://127.0.0.1:8080/admin/api/health |
      jq -e '.status == "ok"' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "Blueclaw API did not become healthy before probe setup" >&2
  return 1
}

reset_chromium_state() {
  pkill -f chromium 2>/dev/null || true
  pkill -f chrome 2>/dev/null || true
  sleep 1
}

site_capture_chroot_base=""

prepare_rootfs_browser_chroot() {
  local rootfs_image="/opt/internkim/blueclaw-runtime/rootfs.ext4"
  [ -f "$rootfs_image" ] || return 1
  site_capture_chroot_base="$(mktemp -d /tmp/internkim-site-browser-XXXXXX)"
  mkdir -p "$site_capture_chroot_base/lower" "$site_capture_chroot_base/upper" "$site_capture_chroot_base/work" "$site_capture_chroot_base/root"
  mount -o loop,ro,noload "$rootfs_image" "$site_capture_chroot_base/lower" || return 1
  mount -t overlay overlay \
    -o "lowerdir=$site_capture_chroot_base/lower,upperdir=$site_capture_chroot_base/upper,workdir=$site_capture_chroot_base/work" \
    "$site_capture_chroot_base/root" || return 1
  mount --bind /proc "$site_capture_chroot_base/root/proc" || return 1
  mount --bind /dev "$site_capture_chroot_base/root/dev" || return 1
  mount --bind /tmp "$site_capture_chroot_base/root/tmp" || return 1
  rm -f "$site_capture_chroot_base/root/etc/resolv.conf"
  cat /etc/resolv.conf > "$site_capture_chroot_base/root/etc/resolv.conf" 2>/dev/null || true
  return 0
}

cleanup_rootfs_browser_chroot() {
  [ -n "$site_capture_chroot_base" ] || return 0
  umount "$site_capture_chroot_base/root/tmp" 2>/dev/null || true
  umount "$site_capture_chroot_base/root/dev" 2>/dev/null || true
  umount "$site_capture_chroot_base/root/proc" 2>/dev/null || true
  umount "$site_capture_chroot_base/root" 2>/dev/null || true
  umount "$site_capture_chroot_base/lower" 2>/dev/null || true
  rm -rf "$site_capture_chroot_base"
  site_capture_chroot_base=""
}

run_site_browser() {
  if [ -n "$site_capture_chroot_base" ]; then
    timeout 45s chroot "$site_capture_chroot_base/root" /usr/bin/chromium "$@"
    return $?
  fi
  timeout 45s "$site_browser_path" "$@"
}

launch_chromium_capture() {
  local profile_dir
  profile_dir="$(mktemp -d)"
  run_site_browser --headless --no-sandbox --disable-gpu --disable-dev-shm-usage \
    --user-data-dir="$profile_dir" --virtual-time-budget=8000 "$@"
  local exit_code=$?
  rm -rf "$profile_dir"
  return "$exit_code"
}

capture_site_screenshots() {
  local public_url="$1"
  reset_chromium_state
  site_browser_path="$(command -v chromium || command -v chromium-browser || command -v google-chrome || command -v google-chrome-stable || true)"
  if [ -z "$site_browser_path" ] && ! prepare_rootfs_browser_chroot; then
    cleanup_rootfs_browser_chroot
    echo "expected Chromium-compatible browser or Blueclaw runtime rootfs image for site screenshot verification" >&2
    return 1
  fi
  desktop_screenshot_file="/tmp/internkim-site-$timestamp-desktop.png"
  mobile_screenshot_file="/tmp/internkim-site-$timestamp-mobile.png"
  site_screenshot_desktop_log="/tmp/internkim-site-screenshot-desktop-$timestamp.log"
  site_screenshot_mobile_log="/tmp/internkim-site-screenshot-mobile-$timestamp.log"
  site_dom_log="/tmp/internkim-site-dom-$timestamp.log"
  launch_chromium_capture \
    --hide-scrollbars --window-size=1440,1000 --screenshot="$desktop_screenshot_file" "$public_url" \
    >"$site_screenshot_desktop_log" 2>&1 || true
  launch_chromium_capture \
    --hide-scrollbars --window-size=390,900 --screenshot="$mobile_screenshot_file" "$public_url" \
    >"$site_screenshot_mobile_log" 2>&1 || true
  launch_chromium_capture \
    --window-size=1440,1000 --dump-dom "$public_url" \
    > "$site_dom_file" 2>"$site_dom_log" || true
  cleanup_rootfs_browser_chroot
  python3 - "$site_dom_file" "$site_html_raw_file" <<'PY'
from pathlib import Path
import sys

MAX_HTML_RAW_BYTES = 2_000_000
raw_bytes = Path(sys.argv[1]).read_bytes()[:MAX_HTML_RAW_BYTES]
Path(sys.argv[2]).write_text(raw_bytes.decode("utf-8", errors="replace"), encoding="utf-8")
PY
  python3 - "$site_dom_file" > "$site_text_file" <<'PY'
from html.parser import HTMLParser
from pathlib import Path
import re
import sys

class TextParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.parts = []
        self.is_hidden = False

    def handle_starttag(self, tag, attrs):
        if tag in {"script", "style", "svg"}:
            self.is_hidden = True

    def handle_endtag(self, tag):
        if tag in {"script", "style", "svg"}:
            self.is_hidden = False

    def handle_data(self, data):
        if not self.is_hidden and data.strip():
            self.parts.append(data.strip())

parser = TextParser()
parser.feed(Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace"))
print(re.sub(r"\s+", " ", " ".join(parser.parts)).strip()[:50000])
PY
  if ! python3 - "$site_dom_file" "$public_url" "$site_css_raw_file" > "$site_style_metrics_file" <<'PY'
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urljoin
from urllib.request import urlopen
import json
import re
import statistics
import sys

class StyleParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []
        self.styles = []
        self.is_style = False

    def handle_starttag(self, tag, attrs):
        attributes = {name.lower(): value or "" for name, value in attrs}
        if tag == "style":
            self.is_style = True
        if tag == "link" and "stylesheet" in attributes.get("rel", "").lower() and attributes.get("href"):
            self.links.append(attributes["href"])

    def handle_endtag(self, tag):
        if tag == "style":
            self.is_style = False

    def handle_data(self, data):
        if self.is_style:
            self.styles.append(data)

def css_number_values(source, property_names):
    values = []
    property_pattern = "|".join(re.escape(name) for name in property_names)
    for match in re.finditer(r"(?:" + property_pattern + r")\s*:\s*([^;}{]+)", source, re.IGNORECASE):
        declaration = match.group(1)
        for number, unit in re.findall(r"(-?\d+(?:\.\d+)?)\s*(px|rem|em)?", declaration, re.IGNORECASE):
            value = float(number)
            normalized_unit = unit.lower()
            if normalized_unit in {"rem", "em"}:
                value *= 16
            values.append(value)
    return values

def unitless_line_height_values(source):
    values = []
    for match in re.finditer(r"line-height\s*:\s*([^;}{]+)", source, re.IGNORECASE):
        declaration = match.group(1)
        if re.search(r"px|rem|em|calc|clamp", declaration, re.IGNORECASE):
            continue
        for number in re.findall(r"-?\d+(?:\.\d+)?", declaration):
            values.append(float(number))
    return values

def summarize(values):
    if not values:
        return {"count": 0}
    ordered = sorted(values)
    return {
        "count": len(ordered),
        "min": round(ordered[0], 3),
        "median": round(statistics.median(ordered), 3),
        "max": round(ordered[-1], 3),
    }

def score_metrics(font_sizes, line_heights_px, line_heights_unitless, spacing_values, has_korean_font, has_system_font):
    checks = []
    if font_sizes:
        median_font_size = statistics.median(font_sizes)
        checks.append(1.0 if 13 <= median_font_size <= 18 else 0.5)
        checks.append(1.0 if max(font_sizes) <= 72 else 0.65)
    else:
        checks.append(0.55)
    if line_heights_unitless:
        median_line_height = statistics.median(line_heights_unitless)
        checks.append(1.0 if 1.25 <= median_line_height <= 1.8 else 0.55)
    elif line_heights_px:
        median_line_height = statistics.median(line_heights_px)
        checks.append(1.0 if 18 <= median_line_height <= 32 else 0.55)
    else:
        checks.append(0.65)
    if spacing_values:
        median_spacing = statistics.median(spacing_values)
        checks.append(1.0 if 8 <= median_spacing <= 80 and max(spacing_values) <= 260 else 0.6)
    else:
        checks.append(0.6)
    checks.append(1.0 if has_korean_font or has_system_font else 0.6)
    return round(sum(checks) / len(checks), 3)

def fetch_stylesheet(url):
    try:
        with urlopen(url, timeout=8) as response:
            return response.read(200000).decode("utf-8", errors="replace")
    except Exception:
        return ""

document = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace")
public_url = sys.argv[2]
parser = StyleParser()
parser.feed(document)
linked_css = [fetch_stylesheet(urljoin(public_url, href)) for href in parser.links[:8]]
css = "\n".join(parser.styles + linked_css)
MAX_CSS_RAW_BYTES = 512_000
Path(sys.argv[3]).write_bytes(css.encode("utf-8", errors="replace")[:MAX_CSS_RAW_BYTES])
font_families = sorted(set(re.findall(r"font-family\s*:\s*([^;}{]+)", css, re.IGNORECASE)))[:20]
korean_font_pattern = re.compile(r"Apple SD Gothic|Malgun|Noto Sans KR|Pretendard|Paperlogy|Nanum|Spoqa", re.IGNORECASE)
system_font_pattern = re.compile(r"system-ui|-apple-system|BlinkMacSystemFont|sans-serif", re.IGNORECASE)
font_sizes = css_number_values(css, ["font-size"])
line_heights_px = css_number_values(css, ["line-height"])
line_heights_unitless = unitless_line_height_values(css)
spacing_values = css_number_values(css, ["margin", "margin-top", "margin-right", "margin-bottom", "margin-left", "padding", "padding-top", "padding-right", "padding-bottom", "padding-left", "gap", "row-gap", "column-gap"])
has_korean_font = any(korean_font_pattern.search(value) for value in font_families)
has_system_font = any(system_font_pattern.search(value) for value in font_families)
print(json.dumps({
    "cssBytes": len(css.encode("utf-8")),
    "stylesheetCount": len([value for value in linked_css if value.strip()]),
    "inlineStyleBlockCount": len(parser.styles),
    "fontFamilies": font_families,
    "hasKoreanFont": has_korean_font,
    "hasSystemFont": has_system_font,
    "fontSizePixels": summarize(font_sizes),
    "lineHeightPixels": summarize(line_heights_px),
    "lineHeightUnitless": summarize(line_heights_unitless),
    "spacingPixels": summarize(spacing_values),
    "typographyScore": score_metrics(font_sizes, line_heights_px, line_heights_unitless, spacing_values, has_korean_font, has_system_font),
}, ensure_ascii=False))
PY
  then
    printf '{}' > "$site_style_metrics_file"
  fi
  if test -s "$desktop_screenshot_file" && test -s "$mobile_screenshot_file" && test -s "$site_text_file"; then
    return 0
  fi
  echo "site screenshot capture failed; chromium diagnostics follow" >&2
  echo "--- desktop screenshot log ($site_screenshot_desktop_log) ---" >&2
  tail -c 4000 "$site_screenshot_desktop_log" >&2 2>/dev/null || true
  echo "--- mobile screenshot log ($site_screenshot_mobile_log) ---" >&2
  tail -c 4000 "$site_screenshot_mobile_log" >&2 2>/dev/null || true
  echo "--- dump-dom log ($site_dom_log) ---" >&2
  tail -c 4000 "$site_dom_log" >&2 2>/dev/null || true
  return 1
}

write_site_screenshots_json() {
  printf '[]' > "$site_screenshots_file"
  if [ "$site_screenshots_verified" != "true" ]; then
    return 0
  fi
  local desktop_base64_file
  local mobile_base64_file
  desktop_base64_file="$(mktemp)"
  mobile_base64_file="$(mktemp)"
  base64 -w 0 "$desktop_screenshot_file" > "$desktop_base64_file" 2>/dev/null || base64 "$desktop_screenshot_file" | tr -d '\n' > "$desktop_base64_file"
  base64 -w 0 "$mobile_screenshot_file" > "$mobile_base64_file" 2>/dev/null || base64 "$mobile_screenshot_file" | tr -d '\n' > "$mobile_base64_file"
  jq -cn \
    --rawfile desktop_content "$desktop_base64_file" \
    --rawfile mobile_content "$mobile_base64_file" \
    '[
      {fileID:"site-desktop", filename:"site-desktop.png", contentType:"image/png", contentBase64:$desktop_content},
      {fileID:"site-mobile", filename:"site-mobile.png", contentType:"image/png", contentBase64:$mobile_content}
    ]' > "$site_screenshots_file"
  rm -f "$desktop_base64_file" "$mobile_base64_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  if [ -n "${bot_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $mattermost_token" \
      "http://localhost:8065/api/v4/posts/$bot_post_id" >/dev/null || true
  fi
  if [ -n "${user_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $user_token" \
      "http://localhost:8065/api/v4/posts/$user_post_id" >/dev/null || true
  fi
  if [ -n "${user_id:-}" ]; then
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
      curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
        "http://localhost:8065/api/v4/users/$user_id" >/dev/null || true
  fi
  if [ -n "${channel_id:-}" ]; then
    site_list_file="$(mktemp)"
    if curl --silent --show-error http://127.0.0.1:8080/admin/api/sites -o "$site_list_file"; then
      site_ids="$(jq -r --arg conversation_id "$channel_id" '.sites[]? | select(.conversationID == $conversation_id) | .siteID' "$site_list_file" 2>/dev/null || true)"
    else
      site_ids=""
    fi
    rm -f "$site_list_file"
    for site_id in $site_ids; do
      curl --silent --show-error -X DELETE -H "Content-Type: application/json" \
        -d '{"confirm":"DELETE","userConfirmed":true}' \
        "http://127.0.0.1:8080/admin/api/sites/$site_id" >/dev/null || true
    done
  fi
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
}
trap cleanup EXIT

delete_probe_user() {
  local probe_user_id="$1"
  local probe_email="$2"
  if [ -n "$probe_user_id" ]; then
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$probe_user_id?permanent=true" >/dev/null 2>&1 || \
      curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
        "http://localhost:8065/api/v4/users/$probe_user_id" >/dev/null || true
  fi
  if [ -n "$probe_email" ]; then
    curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$probe_email" >/dev/null || true
  fi
}

delete_stale_probe_users() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  api_request "search stale probe users" POST http://localhost:8065/api/v4/users/search "$admin_token" \
    "$(jq -cn --arg term probe-mattermost '{term:$term}')" |
    jq -r '.[] | select((.email // "") | startswith("probe-mattermost-")) | [.id, .email] | @tsv' |
    while IFS="$(printf '\t')" read -r probe_user_id probe_email; do
      delete_probe_user "$probe_user_id" "$probe_email"
    done
}

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token")"
bot_user_id="$(printf '%%s' "$bot_user" | jq -r '.id // empty')"
bot_username="$(printf '%%s' "$bot_user" | jq -r '.username // empty')"
test -n "$bot_user_id"

delete_stale_probe_users

wait_for_blueclaw_health

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(api_request "create probe user" POST http://localhost:8065/api/v4/users "$admin_token" "$user_body" | jq -r '.id')"
test -n "$user_id"

user_login_headers="$(mktemp)"
user_login_body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-probe-user-login.json \
  -H "Content-Type: application/json" \
  -d "$user_login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

blueclaw_request "invite probe user" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$user_id" --arg email "$email" '{personID:$personID,email:$email}')" >/dev/null

channel_id="$(api_request "create probe dm" POST http://localhost:8065/api/v4/channels/direct "$user_token" \
  "$(jq -cn --arg user_id "$user_id" --arg bot_user_id "$bot_user_id" '[$user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$channel_id"

post_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')"
user_post="$(api_request "post probe message" POST http://localhost:8065/api/v4/posts "$user_token" "$post_body")"
user_post_id="$(printf '%%s' "$user_post" | jq -r '.id')"
user_post_create_at="$(printf '%%s' "$user_post" | jq -r '.create_at')"
test -n "$user_post_id"

bot_post_id=""
for _ in $(seq 1 "$reply_timeout_seconds"); do
  bot_post_id="$(api_request "wait for probe reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$user_post_create_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | .id' | head -1)"
  if [ -n "$bot_post_id" ]; then
    break
  fi
  sleep 1
done
if [ -z "$bot_post_id" ]; then
  echo "expected Mattermost bot reply for probe post $user_post_id" >&2
  echo "probe channel: $channel_id" >&2
  echo "probe bot membership status: $(curl --silent --show-error --output /tmp/internkim-probe-bot-member.json --write-out '%%{http_code}' -H "Authorization: Bearer $mattermost_token" "http://localhost:8065/api/v4/channels/$channel_id/members/$bot_user_id" || true)" >&2
  echo "probe bot channel posts status: $(curl --silent --show-error --output /tmp/internkim-probe-bot-posts.json --write-out '%%{http_code}' -H "Authorization: Bearer $mattermost_token" "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=5" || true)" >&2
  jq -r '.order // [] | @json' /tmp/internkim-probe-bot-posts.json >&2 || true
  echo "probe bot channel list contains channel: $(curl --silent --show-error -H "Authorization: Bearer $mattermost_token" "http://localhost:8065/api/v4/users/$bot_user_id/channels" | jq -r --arg channel_id "$channel_id" 'map(.id == $channel_id) | any')" >&2
  blueclaw_request "probe task list after missing reply" GET http://127.0.0.1:8080/admin/api/task |
    jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | @json' >&2 || true
  probe_task_run_id="$(blueclaw_request "probe task id after missing reply" GET http://127.0.0.1:8080/admin/api/task | jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty')" || true
  if [ -n "$probe_task_run_id" ]; then
    echo "probe task detail (last events) for $probe_task_run_id:" >&2
    blueclaw_request "probe task detail after missing reply" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$probe_task_run_id" |
      jq -r 'def detail: if type == "array" then .[0] else . end; detail | ("status=" + (.taskRun.status // "?")), ((.taskEvents // []) | .[-30:] | .[] | .name + " " + ((.body // "") | tostring | .[0:160]))' >&2 || true
  fi
  exit 1
fi

bot_post_file="$(mktemp)"
downloaded_files_file="$(mktemp)"
printf '[]' > "$downloaded_files_file"
api_request "fetch probe reply" GET "http://localhost:8065/api/v4/posts/$bot_post_id" "$admin_token" > "$bot_post_file"
find_probe_task_run_id() {
  blueclaw_request "find probe task" GET http://127.0.0.1:8080/admin/api/task |
    jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty'
}
fetch_latest_bot_post() {
  latest_bot_post_id="$(api_request "wait for latest bot reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$user_post_create_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | [.create_at, .id] | @tsv' |
    sort -n | tail -1 | awk '{print $2}')"
  if [ -n "$latest_bot_post_id" ]; then
    bot_post_id="$latest_bot_post_id"
    api_request "fetch latest bot reply" GET "http://localhost:8065/api/v4/posts/$bot_post_id" "$admin_token" > "$bot_post_file"
  fi
}
task_run_id="$(find_probe_task_run_id)"
if [ "$wait_for_completion" = "true" ] && [ -z "$task_run_id" ]; then
  for _ in $(seq 1 "$registration_timeout_seconds"); do
    task_run_id="$(find_probe_task_run_id)"
    if [ -n "$task_run_id" ]; then
      break
    fi
    sleep 1
  done
fi
task_detail_file="$(mktemp)"
printf '{}' > "$task_detail_file"
approval_sent=false
if [ -n "$task_run_id" ]; then
  blueclaw_request "probe task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
fi
expected_tool_count="$(printf '%%s' "$expected_tools_json" | jq 'length')"
expected_event_count="$(printf '%%s' "$expected_events_json" | jq 'length')"
should_wait_for_task=false
if [ "$wait_for_completion" = "true" ] || [ "$expect_browser_open" = "true" ] || [ "$expected_tool_count" != "0" ] || [ "$expected_event_count" != "0" ]; then
  should_wait_for_task=true
fi
if [ "$wait_for_completion" = "true" ] && [ -z "$task_run_id" ]; then
  echo "expected a task for probe prompt before waiting for completion" >&2
  exit 1
fi
if [ -n "$task_run_id" ] && [ "$should_wait_for_task" = "true" ]; then
  for _ in $(seq 1 "$completion_timeout_seconds"); do
    blueclaw_request "probe task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
    if [ "$auto_confirm" = "true" ] && [ "$approval_sent" = "false" ] && jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "confirmation.requested")' "$task_detail_file" >/dev/null; then
      approval_body="$(jq -cn --arg channel_id "$channel_id" --arg root_id "$user_post_id" '{channel_id:$channel_id,root_id:$root_id,message:"해"}')"
      api_request "post probe approval" POST http://localhost:8065/api/v4/posts "$user_token" "$approval_body" >/dev/null
      approval_sent=true
      sleep 1
      continue
    fi
    task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
    if [ "$task_status" = "completed" ] || [ "$task_status" = "blocked" ] || [ "$task_status" = "failed" ] || [ "$task_status" = "cancelled" ]; then
      break
    fi
    sleep 1
  done
fi
browser_open_verified=false
desktop_screenshot_file=""
mobile_screenshot_file=""
site_screenshots_verified=false
public_url=""
site_dom_file="$(mktemp)"
site_text_file="$(mktemp)"
site_html_raw_file="$(mktemp)"
site_css_raw_file="$(mktemp)"
site_style_metrics_file="$(mktemp)"
site_screenshots_file="$(mktemp)"
printf '{}' > "$site_style_metrics_file"
printf '[]' > "$site_screenshots_file"
if [ "$expect_browser_open" = "true" ]; then
  if [ -z "$task_run_id" ]; then
    echo "expected successful browser.open result, but no task was created for probe prompt" >&2
    exit 1
  fi
  if jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "tool.browser.open.result" and (((.body // "{}") | fromjson? // {}) | .isError != true))' "$task_detail_file" >/dev/null; then
    browser_open_verified=true
  else
    echo "expected successful tool.browser.open.result for probe task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
fi

for expected_tool in $(printf '%%s' "$expected_tools_json" | jq -r '.[]'); do
  if [ -z "$task_run_id" ]; then
    echo "expected tool.$expected_tool.requested, but no task was created for probe prompt" >&2
    exit 1
  fi
  if ! jq -e --arg name "tool.$expected_tool.requested" --arg fragment "$expected_tool" --arg operation "\"operation\":\"$expected_tool\"" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; ((.name == $name) and ((.body // "") | tostring | contains($fragment))) or ((.name == "tool.capability.invoke.requested") and ((.body // "") | tostring | contains($operation))))' "$task_detail_file" >/dev/null; then
    echo "expected requested tool event for $expected_tool in task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

for expected_event in $(printf '%%s' "$expected_events_json" | jq -r '.[]'); do
  if [ -z "$task_run_id" ]; then
    echo "expected event $expected_event, but no task was created for probe prompt" >&2
    exit 1
  fi
  if ! jq -e --arg name "$expected_event" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == $name)' "$task_detail_file" >/dev/null; then
    echo "expected task event $expected_event in task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

if [ -n "$task_run_id" ] && [ "$should_wait_for_task" = "true" ]; then
  task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
  if [ "$task_status" != "completed" ]; then
    echo "expected completed task for probe prompt, got ${task_status:-unknown} in task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
fi
fetch_latest_bot_post
download_bot_files

emit_site_verification_result() {
  local is_ok="$1"
  local failure_reason="${2:-}"
  jq -cn \
    --argjson ok "$is_ok" \
    --arg failure_reason "$failure_reason" \
    --arg channel_id "$channel_id" \
    --arg user_post_id "$user_post_id" \
    --arg bot_post_id "$bot_post_id" \
    --arg task_run_id "$task_run_id" \
    --arg site_public_url "$public_url" \
    --arg desktop_screenshot_file "$desktop_screenshot_file" \
    --arg mobile_screenshot_file "$mobile_screenshot_file" \
    --argjson keep "$keep_artifacts" \
    --argjson browser_open_verified "$browser_open_verified" \
    --argjson site_screenshots_verified "$site_screenshots_verified" \
    --argjson auto_confirmation_sent "$approval_sent" \
    --rawfile site_html_text "$site_text_file" \
    --rawfile site_html_raw "$site_html_raw_file" \
    --rawfile site_css_raw "$site_css_raw_file" \
    --slurpfile bot_post "$bot_post_file" \
    --slurpfile downloaded_files "$downloaded_files_file" \
    --slurpfile site_screenshots "$site_screenshots_file" \
    --slurpfile site_style_metrics "$site_style_metrics_file" \
    --slurpfile task_detail "$task_detail_file" \
    '{
      ok: $ok,
      failureReason: $failure_reason,
      kept: $keep,
      channelID: $channel_id,
      userPostID: $user_post_id,
      botPostID: $bot_post_id,
      taskRunID: $task_run_id,
      browserOpenVerified: $browser_open_verified,
      siteScreenshotsVerified: $site_screenshots_verified,
      autoConfirmationSent: $auto_confirmation_sent,
      sitePublicURL: $site_public_url,
      siteHTMLText: $site_html_text,
      siteHTMLRaw: $site_html_raw,
      siteCSSRaw: $site_css_raw,
      siteStyleMetrics: ($site_style_metrics[0] // {}),
      siteScreenshotFiles: [$desktop_screenshot_file, $mobile_screenshot_file],
      botMessage: $bot_post[0].message,
      fileIDs: ($bot_post[0].file_ids // []),
      downloadedFiles: ($downloaded_files[0] // []),
      siteScreenshots: ($site_screenshots[0] // []),
      taskStatus: ((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskRun.status // null),
      taskEvents: (((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskEvents // []) | map({name, body: ((.body // "") | tostring | .[0:1200])}))
    }'
}

emit_site_verification_failure() {
  local reason="$1"
  echo "$reason" >&2
  emit_site_verification_result false "$reason"
  exit 1
}

if [ "$expect_public_url" = "true" ]; then
  public_url_verified=false
  public_html_file="$(mktemp)"
  for _ in $(seq 1 "$public_url_timeout_seconds"); do
    bot_post_id="$(api_request "wait for final site reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
      jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$user_post_create_at" \
        '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | [.create_at, .id] | @tsv' |
      sort -n | tail -1 | awk '{print $2}')"
    if [ -n "$bot_post_id" ]; then
      api_request "fetch final site reply" GET "http://localhost:8065/api/v4/posts/$bot_post_id" "$admin_token" > "$bot_post_file"
      bot_message="$(jq -r '.message // ""' "$bot_post_file")"
      if printf '%%s\n' "$bot_message" | grep -Eiq 'bun 없음|외부 호스팅|관리자 점검|Quality Gate 기준 미충족|품질 검사 기준.*완성하지 못'; then
        emit_site_verification_failure "site deploy final reply contained a generic infrastructure excuse: $bot_message"
      fi
      public_url="$(printf '%%s\n' "$bot_message" | grep -Eo 'https://[^[:space:])>]+' | sed -E 's/[).,;:!?*]+$//' | grep 'intern\.kim' | head -1 || true)"
      if [ -n "$public_url" ]; then
        if curl --location --fail --silent --show-error --max-time 30 "$public_url" -o "$public_html_file"; then
          if ! grep -Fq 'Sorry, we could not find the page.' "$public_html_file" &&
             ! grep -Fq 'INTERNKIM_SITE_STARTER_REPLACE_ME' "$public_html_file" &&
             ! grep -Fq 'Replace this starter' "$public_html_file" &&
             [ "$(wc -c < "$public_html_file")" -gt 200 ]; then
            public_url_verified=true
            break
          fi
        fi
      fi
    fi
    sleep 1
  done
  if [ -z "$public_url" ]; then
    jq -r '.message // ""' "$bot_post_file" >&2 || true
    emit_site_verification_failure "expected final site public URL in Mattermost bot reply"
  fi
  if [ "$public_url_verified" != "true" ]; then
    emit_site_verification_failure "site public URL did not return valid HTML: $public_url"
  fi
  if capture_site_screenshots "$public_url"; then
    site_screenshots_verified=true
    write_site_screenshots_json
  else
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | .[-10:] | map({name, body})' "$task_detail_file" >&2 || true
    emit_site_verification_failure "site public URL could not be verified with browser screenshots: $public_url"
  fi
  if grep -Fq 'Sorry, we could not find the page.' "$public_html_file"; then
    emit_site_verification_failure "site public URL returned not-found page: $public_url"
  fi
  if grep -Fq 'INTERNKIM_SITE_STARTER_REPLACE_ME' "$public_html_file" || grep -Fq 'Replace this starter' "$public_html_file"; then
    emit_site_verification_failure "site public URL returned starter scaffold instead of requested content: $public_url"
  fi
fi

emit_site_verification_result true ""
`, strconv.Quote(encodedPrompt), strconv.Quote(encodedExpectedTools), strconv.Quote(encodedExpectedEvents), keepValue, timeoutSeconds, expectBrowserOpenValue, expectPublicURLValue, downloadFilesValue, waitForCompletionValue, autoConfirmValue)
}

func verifyMattermostDirectMessageE2EScript(keep bool, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

timestamp="$(date +%%s)"
requester_email="probe-mattermost-dm-requester-$timestamp@internkim.test"
requester_username="probedmreq$timestamp"
recipient_email="probe-mattermost-dm-recipient-$timestamp@internkim.test"
recipient_username="probedmto$timestamp"
password="ProbePass!$timestamp-InternKim-Mattermost"
target_message="InternKim DM E2E $timestamp"
prompt="${recipient_username}에게 ${target_message}라고 DM 보내줘"
keep_artifacts=%s
timeout_seconds=%d
test_started_at="$(date +%%s%%3N)"

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

wait_for_blueclaw_health() {
  for _ in $(seq 1 "$timeout_seconds"); do
    if curl --silent --show-error --fail --max-time 15 http://127.0.0.1:8080/admin/api/health |
      jq -e '.status == "ok"' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "Blueclaw API did not become healthy before direct-message E2E" >&2
  return 1
}

login_user() {
  local username="$1"
  local headers
  headers="$(mktemp)"
  curl --silent --show-error --fail -D "$headers" -o /tmp/internkim-dm-user-login.json \
    -H "Content-Type: application/json" \
    -d "$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')" \
    http://localhost:8065/api/v4/users/login >/dev/null
  awk 'tolower($1) == "token:" {print $2}' "$headers" | tr -d '\r'
}

create_user() {
  local email="$1"
  local username="$2"
  api_request "create user $username" POST http://localhost:8065/api/v4/users "$admin_token" \
    "$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')" |
    jq -r '.id'
}

delete_user() {
  local user_id="$1"
  if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
    return 0
  fi
  curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id" >/dev/null || \
    echo "cleanup warning: failed to delete Mattermost user $user_id" >&2
}

delete_post() {
  local token="$1"
  local post_id="$2"
  if [ -z "$post_id" ] || [ "$post_id" = "null" ]; then
    return 0
  fi
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $token" \
    "http://localhost:8065/api/v4/posts/$post_id" >/dev/null || echo "cleanup warning: failed to delete Mattermost post $post_id" >&2
}

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  delete_post "${requester_token:-}" "${request_post_id:-}"
  delete_post "${requester_token:-}" "${approval_post_id:-}"
  delete_post "${mattermost_token:-}" "${bot_reply_post_id:-}"
  delete_post "${mattermost_token:-}" "${recipient_dm_post_id:-}"
  delete_user "${requester_user_id:-}"
  delete_user "${recipient_user_id:-}"
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$requester_email" >/dev/null || true
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$recipient_email" >/dev/null || true
}
trap cleanup EXIT

admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
admin_headers="$(mktemp)"
curl --silent --show-error --fail -D "$admin_headers" -o /tmp/internkim-dm-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$admin_headers" | tr -d '\r')"
test -n "$admin_token"

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token" | jq -r '.id // empty')"
test -n "$bot_user_id"
wait_for_blueclaw_health

requester_user_id="$(create_user "$requester_email" "$requester_username")"
recipient_user_id="$(create_user "$recipient_email" "$recipient_username")"
test -n "$requester_user_id"
test -n "$recipient_user_id"
requester_token="$(login_user "$requester_username")"
recipient_token="$(login_user "$recipient_username")"
test -n "$requester_token"
test -n "$recipient_token"

blueclaw_request "invite requester" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$requester_user_id" --arg email "$requester_email" '{personID:$personID,email:$email}')" >/dev/null
blueclaw_request "invite recipient" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$recipient_user_id" --arg email "$recipient_email" '{personID:$personID,email:$email}')" >/dev/null

request_channel_id="$(api_request "create requester dm" POST http://localhost:8065/api/v4/channels/direct "$requester_token" \
  "$(jq -cn --arg requester_user_id "$requester_user_id" --arg bot_user_id "$bot_user_id" '[$requester_user_id,$bot_user_id]')" | jq -r '.id')"
recipient_channel_id="$(api_request "create recipient dm" POST http://localhost:8065/api/v4/channels/direct "$recipient_token" \
  "$(jq -cn --arg recipient_user_id "$recipient_user_id" --arg bot_user_id "$bot_user_id" '[$recipient_user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$request_channel_id"
test -n "$recipient_channel_id"

request_post="$(api_request "post direct-message request" POST http://localhost:8065/api/v4/posts "$requester_token" \
  "$(jq -cn --arg channel_id "$request_channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')")"
request_post_id="$(printf '%%s' "$request_post" | jq -r '.id')"
request_post_create_at="$(printf '%%s' "$request_post" | jq -r '.create_at')"
test -n "$request_post_id"

find_task_id() {
  blueclaw_request "find direct-message task" GET http://127.0.0.1:8080/admin/api/task |
    jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty'
}

task_detail_file="$(mktemp)"
approval_post_id=""
task_run_id=""
for _ in $(seq 1 "$timeout_seconds"); do
  task_run_id="$(find_task_id)"
  if [ -n "$task_run_id" ]; then
    blueclaw_request "direct-message task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
    if [ -z "$approval_post_id" ] && jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "confirmation.requested")' "$task_detail_file" >/dev/null; then
      approval_post="$(api_request "post direct-message approval" POST http://localhost:8065/api/v4/posts "$requester_token" \
        "$(jq -cn --arg channel_id "$request_channel_id" --arg root_id "$request_post_id" '{channel_id:$channel_id,root_id:$root_id,message:"해"}')")"
      approval_post_id="$(printf '%%s' "$approval_post" | jq -r '.id')"
    fi
    task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
    if [ "$task_status" = "completed" ] || [ "$task_status" = "failed" ] || [ "$task_status" = "blocked" ] || [ "$task_status" = "cancelled" ]; then
      break
    fi
  fi
  sleep 1
done

if [ -z "$task_run_id" ]; then
  echo "direct-message E2E did not create a task" >&2
  exit 1
fi
blueclaw_request "final direct-message task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
if [ "$task_status" != "completed" ]; then
  echo "direct-message E2E task did not complete: $task_status ($task_run_id)" >&2
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi
if ! jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "tool.message.send.requested")' "$task_detail_file" >/dev/null; then
  echo "expected message.send request in direct-message E2E task $task_run_id" >&2
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi
if jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; (.name // "") | contains("platform.dm.send"))' "$task_detail_file" >/dev/null; then
  echo "legacy platform.dm.send appeared in direct-message E2E task $task_run_id" >&2
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi

recipient_dm_post_id=""
for _ in $(seq 1 "$timeout_seconds"); do
  recipient_dm_post_id="$(api_request "wait for recipient dm" GET "http://localhost:8065/api/v4/channels/$recipient_channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --arg target_message "$target_message" --argjson posted_after "$request_post_create_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after and (.message | contains($target_message))) | .id' | head -1)"
  if [ -n "$recipient_dm_post_id" ]; then
    break
  fi
  sleep 1
done
if [ -z "$recipient_dm_post_id" ]; then
  echo "expected recipient DM post containing: $target_message" >&2
  api_request "diagnose recipient dm posts" GET "http://localhost:8065/api/v4/channels/$recipient_channel_id/posts?per_page=20" "$admin_token" |
    jq -r '.order as $order | $order[] as $id | .posts[$id] | {id,user_id,message,root_id,create_at,delete_at}' >&2 || true
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi

bot_reply_post_id="$(api_request "fetch requester replies" GET "http://localhost:8065/api/v4/channels/$request_channel_id/posts?per_page=60" "$admin_token" |
  jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$request_post_create_at" \
    '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | [.create_at, .id] | @tsv' |
  sort -n | tail -1 | awk '{print $2}')"

jq -cn \
  --arg task_run_id "$task_run_id" \
  --arg requester_channel_id "$request_channel_id" \
  --arg recipient_channel_id "$recipient_channel_id" \
  --arg recipient_dm_post_id "$recipient_dm_post_id" \
  --argjson keep "$keep_artifacts" \
  --slurpfile task_detail "$task_detail_file" \
  '{
    ok: true,
    kept: $keep,
    taskRunID: $task_run_id,
    requesterChannelID: $requester_channel_id,
    recipientChannelID: $recipient_channel_id,
    recipientDMPostID: $recipient_dm_post_id,
    taskEvents: (((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskEvents // []) | map({name, body: ((.body // "") | tostring | .[0:1200])}))
  }'
`, keepValue, timeoutSeconds)
}

func verifyMattermostMessageDeleteE2EScript(keep bool, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

timestamp="$(date +%%s)"
email="probe-mattermost-delete-$timestamp@internkim.test"
username="probedelete$timestamp"
password="ProbePass!$timestamp-InternKim-Mattermost"
marker="internkim-delete-e2e-$timestamp"
target_count=24
prompt="방금 김인턴이 보낸 $marker 테스트 메시지들을 모두 삭제해줘. 내가 보낸 $marker 메시지는 삭제하지 마."
keep_artifacts=%s
timeout_seconds=%d

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

wait_for_blueclaw_health() {
  for _ in $(seq 1 "$timeout_seconds"); do
    if curl --silent --show-error --fail --max-time 15 http://127.0.0.1:8080/admin/api/health |
      jq -e '.status == "ok"' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "Blueclaw API did not become healthy before message delete E2E setup" >&2
  return 1
}

find_task_id() {
  blueclaw_request "list message delete E2E tasks" GET http://127.0.0.1:8080/admin/api/task |
    jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty'
}

diagnose_connector_event() {
  local message_id="$1"
  local response_file
  local status
  response_file="$(mktemp)"
  status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
    "http://127.0.0.1:8080/admin/api/connector/events?platform=mattermost&messageID=$message_id&limit=5")" || status="000"
  echo "connector event diagnostics for Mattermost message $message_id returned HTTP $status" >&2
  cat "$response_file" >&2 || true
  echo >&2
  rm -f "$response_file"
}

is_post_deleted() {
  local post_id="$1"
  local response_file
  local status
  response_file="$(mktemp)"
  status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
    -H "Authorization: Bearer $admin_token" "http://localhost:8065/api/v4/posts/$post_id")" || status="000"
  if [ "$status" -ge 400 ]; then
    rm -f "$response_file"
    return 0
  fi
  if jq -e '(.delete_at // 0) > 0' "$response_file" >/dev/null; then
    rm -f "$response_file"
    return 0
  fi
  rm -f "$response_file"
  return 1
}

assert_post_alive() {
  local post_id="$1"
  local label="$2"
  local response_file
  response_file="$(mktemp)"
  api_request "fetch $label" GET "http://localhost:8065/api/v4/posts/$post_id" "$admin_token" > "$response_file"
  if jq -e '(.delete_at // 0) > 0' "$response_file" >/dev/null; then
    echo "$label was unexpectedly deleted: $post_id" >&2
    cat "$response_file" >&2
    rm -f "$response_file"
    exit 1
  fi
  rm -f "$response_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-delete-e2e-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  if [ -n "${channel_id:-}" ]; then
    api_request "cleanup delete E2E posts" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=100" "$admin_token" |
      jq -r --arg marker "$marker" '.posts[] | select((.message // "") | contains($marker)) | .id' |
      while read -r post_id; do
        [ -n "$post_id" ] && curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" "http://localhost:8065/api/v4/posts/$post_id" >/dev/null || true
      done
  fi
  if [ -n "${user_id:-}" ]; then
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
      curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
        "http://localhost:8065/api/v4/users/$user_id" >/dev/null || true
  fi
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
}
trap cleanup EXIT

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token")"
bot_user_id="$(printf '%%s' "$bot_user" | jq -r '.id // empty')"
test -n "$bot_user_id"

wait_for_blueclaw_health

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(api_request "create delete probe user" POST http://localhost:8065/api/v4/users "$admin_token" "$user_body" | jq -r '.id')"
test -n "$user_id"

user_login_headers="$(mktemp)"
user_login_body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-delete-probe-user-login.json \
  -H "Content-Type: application/json" \
  -d "$user_login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

blueclaw_request "invite delete probe user" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$user_id" --arg email "$email" '{personID:$personID,email:$email}')" >/dev/null
if ! blueclaw_request "verify delete probe policy" GET http://127.0.0.1:8080/admin/api/policy |
  jq -e --arg email "$email" 'any(.people[]?; any(.emails[]?; ascii_downcase == $email))' >/dev/null; then
  echo "delete E2E probe user was not present in Blueclaw policy after invite: $email" >&2
  exit 1
fi
identity_body="$(jq -cn --arg senderID "$user_id" '{senderID:$senderID}')"
identity_response="$(curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock \
  -H "Content-Type: application/json" -d "$identity_body" \
  http://internkim/v1/platform/mattermost/identity.resolve)"
if ! printf '%%s' "$identity_response" | jq -e --arg email "$email" '(.email // "" | ascii_downcase) == $email' >/dev/null; then
  echo "delete E2E Mattermost identity resolve did not return the probe email" >&2
  printf '%%s\n' "$identity_response" >&2
  exit 1
fi

channel_id="$(api_request "create delete probe dm" POST http://localhost:8065/api/v4/channels/direct "$user_token" \
  "$(jq -cn --arg user_id "$user_id" --arg bot_user_id "$bot_user_id" '[$user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$channel_id"

bot_post_ids_file="$(mktemp)"
for index in $(seq 1 "$target_count"); do
  bot_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$marker bot-target-$index" '{channel_id:$channel_id,message:$message}')"
  api_request "create bot target $index" POST http://localhost:8065/api/v4/posts "$mattermost_token" "$bot_body" | jq -r '.id' >> "$bot_post_ids_file"
done
test "$(wc -l < "$bot_post_ids_file" | tr -d ' ')" = "$target_count"

prompt_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')"
prompt_post="$(api_request "post delete request" POST http://localhost:8065/api/v4/posts "$user_token" "$prompt_body")"
prompt_post_id="$(printf '%%s' "$prompt_post" | jq -r '.id')"
prompt_post_create_at="$(printf '%%s' "$prompt_post" | jq -r '.create_at')"
test -n "$prompt_post_id"

task_detail_file="$(mktemp)"
approval_sent=false
task_run_id=""
for _ in $(seq 1 "$timeout_seconds"); do
  task_run_id="$(find_task_id)"
  if [ -n "$task_run_id" ]; then
    blueclaw_request "delete E2E task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
    if [ "$approval_sent" = "false" ] && jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "confirmation.requested")' "$task_detail_file" >/dev/null; then
      approval_body="$(jq -cn --arg channel_id "$channel_id" --arg root_id "$prompt_post_id" '{channel_id:$channel_id,root_id:$root_id,message:"해"}')"
      api_request "post delete approval" POST http://localhost:8065/api/v4/posts "$user_token" "$approval_body" >/dev/null
      approval_sent=true
    fi
    task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
    if [ "$task_status" = "completed" ] || [ "$task_status" = "failed" ] || [ "$task_status" = "blocked" ] || [ "$task_status" = "cancelled" ]; then
      break
    fi
  fi
  sleep 1
done

if [ -z "$task_run_id" ]; then
  echo "message delete E2E did not create a task" >&2
  echo "probe identity resolve: $identity_response" >&2
  echo "probe channel: $channel_id prompt post: $prompt_post_id" >&2
  diagnose_connector_event "$prompt_post_id"
  api_request "diagnose delete E2E channel posts" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=20" "$admin_token" |
    jq -r '.order as $order | $order[] as $id | .posts[$id] | {id,user_id,message,root_id,create_at,delete_at}' >&2 || true
  exit 1
fi

blueclaw_request "final delete E2E task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
if [ "$task_status" != "completed" ]; then
  echo "message delete E2E task did not complete: $task_status ($task_run_id)" >&2
  diagnose_connector_event "$prompt_post_id"
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi

for expected_tool in message.search message.delete; do
  if ! jq -e --arg name "tool.$expected_tool.requested" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == $name)' "$task_detail_file" >/dev/null; then
    echo "expected $expected_tool to be requested in message delete E2E task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

for old_tool in platform.dm.inspect platform.dm.send mattermost.post.delete mattermost.post.search mattermost.context.inspect; do
  if jq -e --arg old_tool "$old_tool" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; (.name // "") | contains($old_tool))' "$task_detail_file" >/dev/null; then
    echo "old message tool appeared in message delete E2E task: $old_tool" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

for forbidden_tool in file.read file.preview terminal.run; do
  if jq -e --arg forbidden_tool "$forbidden_tool" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; (.name // "") | contains("tool." + $forbidden_tool + ".requested"))' "$task_detail_file" >/dev/null; then
    echo "message delete E2E leaked into unrelated tool: $forbidden_tool" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

while read -r bot_post_id; do
  if ! is_post_deleted "$bot_post_id"; then
    echo "bot target was not deleted: $bot_post_id" >&2
    exit 1
  fi
done < "$bot_post_ids_file"
assert_post_alive "$prompt_post_id" "user request post"

bot_post_file="$(mktemp)"
latest_bot_post_id="$(api_request "fetch delete E2E bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=100" "$admin_token" |
  jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$prompt_post_create_at" \
    '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | [.create_at, .id] | @tsv' |
  sort -n | tail -1 | awk '{print $2}')"
if [ -n "$latest_bot_post_id" ]; then
  api_request "fetch delete E2E final reply" GET "http://localhost:8065/api/v4/posts/$latest_bot_post_id" "$admin_token" > "$bot_post_file"
else
  printf '{"message":""}' > "$bot_post_file"
fi
if ! jq -e '(.message // "") | test("삭제|deleted|완료")' "$bot_post_file" >/dev/null; then
  echo "final reply did not report deletion completion" >&2
  cat "$bot_post_file" >&2
  exit 1
fi

jq -cn \
  --arg channel_id "$channel_id" \
  --arg task_run_id "$task_run_id" \
  --rawfile deleted_bot_post_ids "$bot_post_ids_file" \
  --arg user_keep_post "$prompt_post_id" \
  --argjson keep "$keep_artifacts" \
  --slurpfile bot_post "$bot_post_file" \
  --slurpfile task_detail "$task_detail_file" \
  '{
    ok: true,
    kept: $keep,
    channelID: $channel_id,
    taskRunID: $task_run_id,
    deletedBotPostIDs: ($deleted_bot_post_ids | split("\n") | map(select(. != ""))),
    preservedUserPostID: $user_keep_post,
    botMessage: ($bot_post[0].message // ""),
    taskEvents: (((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskEvents // []) | map({name, body: ((.body // "") | tostring | .[0:1200])}))
  }'
`, keepValue, timeoutSeconds)
}

func verifyMattermostHTMLAttachmentFollowupScript(keep bool, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

timestamp="$(date +%%s)"
email="probe-mattermost-attachment-$timestamp@internkim.test"
username="probeattach$timestamp"
password="ProbePass!$timestamp-InternKim-Mattermost"
root_prompt="이 HTML 파일 내용 보고 개선점 말해줘. probe-$timestamp"
followup_prompt="다시 시도해보자. probe-$timestamp"
unique_title="InternKim Attachment Preview $timestamp"
keep_artifacts=%s
timeout_seconds=%d
test_started_at="$(date +%%s%%3N)"

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

wait_for_blueclaw_health() {
  for _ in $(seq 1 "$timeout_seconds"); do
    if curl --silent --show-error --fail --max-time 15 http://127.0.0.1:8080/admin/api/health |
      jq -e '.status == "ok"' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "Blueclaw API did not become healthy before attachment probe setup" >&2
  return 1
}

task_id_for_prompt() {
  local prompt="$1"
  blueclaw_request "list tasks for attachment probe" GET http://127.0.0.1:8080/admin/api/task |
    jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty'
}

wait_for_task_attachment_read() {
  local prompt="$1"
  local label="$2"
  local task_id=""
  local task_detail_file
  task_detail_file="$(mktemp)"
  for _ in $(seq 1 "$timeout_seconds"); do
    task_id="$(task_id_for_prompt "$prompt")"
    if [ -n "$task_id" ]; then
      blueclaw_request "$label task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_id" > "$task_detail_file"
      if jq -e --arg title "$unique_title" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; (.name == "tool.file.preview.result" or .name == "tool.file.read.result" or .name == "tool.document.read.result") and ((.body // "") | tostring | contains($title)))' "$task_detail_file" >/dev/null; then
        if jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "tool.terminal.run.requested")' "$task_detail_file" >/dev/null; then
          echo "$label task used terminal.run instead of attachment read tools" >&2
          jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
          return 1
        fi
        task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
        if [ "$task_status" = "completed" ]; then
          printf '%%s' "$task_id"
          return 0
        fi
      fi
      task_status="$(jq -r 'def detail: if type == "array" then .[0] else . end; detail.taskRun.status // empty' "$task_detail_file")"
      if [ "$task_status" = "completed" ]; then
        echo "$label task completed without reading the Mattermost HTML attachment content: $task_id" >&2
        jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
        return 1
      fi
      if [ "$task_status" = "blocked" ] || [ "$task_status" = "failed" ] || [ "$task_status" = "cancelled" ]; then
        echo "$label task ended as $task_status before successful attachment read: $task_id" >&2
        jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
        return 1
      fi
    fi
    sleep 1
  done
  echo "timed out waiting for $label task to read Mattermost HTML attachment" >&2
  return 1
}

latest_bot_message_after() {
  local posted_after="$1"
  api_request "fetch bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=80" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$posted_after" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | [.create_at, .message] | @tsv' |
    sort -n | tail -1 | cut -f2-
}

assert_no_file_excuse() {
  local message="$1"
  local label="$2"
  if printf '%%s\n' "$message" | grep -Eiq '파일을 찾을 수|파일에 접근할 수|다시.*업로드|직접.*공유|내용을 확인하지 못'; then
    echo "$label bot reply still claims the attachment cannot be read: $message" >&2
    return 1
  fi
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-attachment-e2e-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  if [ -n "${channel_id:-}" ]; then
    api_request "cleanup bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=100" "$admin_token" |
      jq -r --arg bot_user_id "${bot_user_id:-}" '.posts[] | select(.user_id == $bot_user_id) | .id' |
      while read -r post_id; do
        [ -n "$post_id" ] && curl --silent --show-error -X DELETE -H "Authorization: Bearer $mattermost_token" "http://localhost:8065/api/v4/posts/$post_id" >/dev/null || true
      done
  fi
  for post_id in "${followup_post_id:-}" "${root_post_id:-}"; do
    [ -n "$post_id" ] && curl --silent --show-error -X DELETE -H "Authorization: Bearer ${user_token:-}" "http://localhost:8065/api/v4/posts/$post_id" >/dev/null || true
  done
  if [ -n "${user_id:-}" ]; then
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
      curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
        "http://localhost:8065/api/v4/users/$user_id" >/dev/null || true
  fi
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
  rm -f "${html_file:-}"
}
trap cleanup EXIT

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token")"
bot_user_id="$(printf '%%s' "$bot_user" | jq -r '.id // empty')"
test -n "$bot_user_id"

wait_for_blueclaw_health

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(api_request "create attachment probe user" POST http://localhost:8065/api/v4/users "$admin_token" "$user_body" | jq -r '.id')"
test -n "$user_id"

user_login_headers="$(mktemp)"
user_login_body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-attachment-probe-user-login.json \
  -H "Content-Type: application/json" \
  -d "$user_login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

blueclaw_request "invite attachment probe user" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$user_id" --arg email "$email" '{personID:$personID,email:$email}')" >/dev/null

channel_id="$(api_request "create attachment probe dm" POST http://localhost:8065/api/v4/channels/direct "$user_token" \
  "$(jq -cn --arg user_id "$user_id" --arg bot_user_id "$bot_user_id" '[$user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$channel_id"

html_file="/tmp/internkim-attachment-probe-$timestamp.html"
printf '<!doctype html><html><body><h1>%%s</h1><p>Automation workflow content for Mattermost attachment E2E.</p></body></html>' "$unique_title" > "$html_file"
upload_response="$(curl --silent --show-error --fail \
  -H "Authorization: Bearer $user_token" \
  -F "channel_id=$channel_id" \
  -F "files=@$html_file;type=text/html;filename=kim-intern-automation-$timestamp.html" \
  http://localhost:8065/api/v4/files)"
file_id="$(printf '%%s' "$upload_response" | jq -r '.file_infos[0].id // empty')"
test -n "$file_id"

root_post_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$root_prompt" --arg file_id "$file_id" '{channel_id:$channel_id,message:$message,file_ids:[$file_id]}')"
root_post="$(api_request "post attachment probe root" POST http://localhost:8065/api/v4/posts "$user_token" "$root_post_body")"
root_post_id="$(printf '%%s' "$root_post" | jq -r '.id')"
root_post_create_at="$(printf '%%s' "$root_post" | jq -r '.create_at')"
test -n "$root_post_id"

root_task_id="$(wait_for_task_attachment_read "$root_prompt" "root attachment")"
root_bot_message="$(latest_bot_message_after "$root_post_create_at")"
assert_no_file_excuse "$root_bot_message" "root attachment"

followup_body="$(jq -cn --arg channel_id "$channel_id" --arg root_id "$root_post_id" --arg message "$followup_prompt" '{channel_id:$channel_id,root_id:$root_id,message:$message}')"
followup_post="$(api_request "post attachment probe followup" POST http://localhost:8065/api/v4/posts "$user_token" "$followup_body")"
followup_post_id="$(printf '%%s' "$followup_post" | jq -r '.id')"
followup_post_create_at="$(printf '%%s' "$followup_post" | jq -r '.create_at')"
test -n "$followup_post_id"

followup_task_id="$(wait_for_task_attachment_read "$followup_prompt" "follow-up attachment")"
followup_bot_message="$(latest_bot_message_after "$followup_post_create_at")"
assert_no_file_excuse "$followup_bot_message" "follow-up attachment"

jq -cn \
  --arg channel_id "$channel_id" \
  --arg root_post_id "$root_post_id" \
  --arg followup_post_id "$followup_post_id" \
  --arg file_id "$file_id" \
  --arg root_task_id "$root_task_id" \
  --arg followup_task_id "$followup_task_id" \
  --arg root_bot_message "$root_bot_message" \
  --arg followup_bot_message "$followup_bot_message" \
  --arg unique_title "$unique_title" \
  --argjson keep "$keep_artifacts" \
  '{ok:true, kept:$keep, channelID:$channel_id, rootPostID:$root_post_id, followupPostID:$followup_post_id, fileID:$file_id, rootTaskID:$root_task_id, followupTaskID:$followup_task_id, uniqueTitle:$unique_title, rootBotMessage:$root_bot_message, followupBotMessage:$followup_bot_message}'
`, keepValue, timeoutSeconds)
}

func prepareMattermostBrowserOpenE2EScript() string {
	return `set -euo pipefail

timestamp="$(date +%s)"
email="probe-browser-open-$timestamp@internkim.test"
username="probebrowser$timestamp"
password="ProbePass!$timestamp-InternKim-Mattermost"

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

admind_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
    -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Admind API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Admind API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-browser-e2e-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

stale_users_file="$(mktemp)"
api_request "search stale probe browser users" POST http://localhost:8065/api/v4/users/search "$admin_token" \
  "$(jq -cn --arg term probe-browser-open '{term:$term}')" > "$stale_users_file"
jq -r '.[] | select((.email // "") | startswith("probe-browser-open-")) | [.id, .email] | @tsv' "$stale_users_file" |
while IFS="$(printf '\t')" read -r stale_user_id stale_email; do
  if [ -n "$stale_user_id" ]; then
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$stale_user_id?permanent=true" >/dev/null 2>&1 || \
      curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
        "http://localhost:8065/api/v4/users/$stale_user_id" >/dev/null || true
  fi
  if [ -n "$stale_email" ]; then
    curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$stale_email" >/dev/null || true
  fi
done

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token" | jq -r '.id // empty')"
test -n "$bot_user_id"

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(api_request "create probe browser user" POST http://localhost:8065/api/v4/users "$admin_token" "$user_body" | jq -r '.id')"
test -n "$user_id"

admind_request "invite probe browser user" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg personID "$user_id" --arg email "$email" '{personID:$personID,email:$email}')" >/dev/null

user_login_headers="$(mktemp)"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-probe-browser-e2e-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

channel_id="$(api_request "create probe browser dm" POST http://localhost:8065/api/v4/channels/direct "$user_token" \
  "$(jq -cn --arg user_id "$user_id" --arg bot_user_id "$bot_user_id" '[$user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$channel_id"

device_url="$(cat /root/.internkim/env/device-url)"
pairing_body="$(jq -cn \
  --arg owner_platform mattermost \
  --arg owner_platform_user_id "$user_id" \
  --arg owner_email "$email" \
  --arg owner_name "$username" \
  --arg device_url "$device_url" \
  '{ownerPlatform:$owner_platform,ownerPlatformUserID:$owner_platform_user_id,ownerEmail:$owner_email,ownerName:$owner_name,deviceURL:$device_url}')"
pairing_document="$(admind_request "create probe companion pairing" POST http://127.0.0.1:18080/_internkim/companion/pairing-codes "$pairing_body")"
pairing_code="$(printf '%s' "$pairing_document" | jq -r '.code // empty')"
test -n "$pairing_code"

jq -cn \
  --arg device_url "$device_url" \
  --arg code "$pairing_code" \
  --arg email "$email" \
  --arg username "$username" \
  --arg password "$password" \
  --arg user_id "$user_id" \
  --arg channel_id "$channel_id" \
  '{deviceURL:$device_url,code:$code,email:$email,username:$username,password:$password,userID:$user_id,channelID:$channel_id}'
`
}

func cleanupMattermostBrowserOpenE2EScript(preparation mattermostBrowserOpenE2EPreparation) string {
	return fmt.Sprintf(`set -euo pipefail

email="$(printf '%%s' %s | base64 -d)"
user_id="$(printf '%%s' %s | base64 -d)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_headers="$(mktemp)"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-browser-e2e-cleanup-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
if [ -n "$user_id" ]; then
  curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id" >/dev/null || true
fi
curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
`, strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Email))), strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.UserID))))
}

func waitMattermostBrowserOpenE2ECompanionOnlineScript(preparation mattermostBrowserOpenE2EPreparation, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	return fmt.Sprintf(`set -euo pipefail

owner_platform_user_id="$(printf '%%s' %s | base64 -d)"
owner_email="$(printf '%%s' %s | base64 -d)"
timeout_seconds=%d
deadline=$((SECONDS + timeout_seconds))

while [ "$SECONDS" -lt "$deadline" ]; do
  status_document="$(curl --silent --show-error http://127.0.0.1:18080/admin/api/companion/status)"
  if printf '%%s' "$status_document" | jq -e --arg owner_platform_user_id "$owner_platform_user_id" --arg owner_email "$owner_email" '
    any(.companions[]?;
      .isOnline == true
      and (.ownerPlatformUserID == $owner_platform_user_id or (.ownerEmail | ascii_downcase) == ($owner_email | ascii_downcase))
      and any(.capabilities[]?; .name == "browser.open")
    )
  ' >/dev/null; then
    echo "remote companion: online with browser.open"
    exit 0
  fi
  sleep 1
done

echo "expected remote companion heartbeat with browser.open for $owner_email" >&2
curl --silent --show-error http://127.0.0.1:18080/admin/api/companion/status |
  jq --arg owner_platform_user_id "$owner_platform_user_id" --arg owner_email "$owner_email" '
    .companions
    | map(select(.ownerPlatformUserID == $owner_platform_user_id or (.ownerEmail | ascii_downcase) == ($owner_email | ascii_downcase)))
    | map({companionID, ownerPlatformUserID, ownerEmail, isOnline, lastSeenAt, capabilityNames: (.capabilities | map(.name))})
  ' >&2
exit 1
`,
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.UserID))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Email))),
		timeoutSeconds,
	)
}

func runMattermostBrowserOpenE2EScript(prompt string, keep bool, timeoutSeconds int, preparation mattermostBrowserOpenE2EPreparation) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

prompt="$(printf '%%s' %s | base64 -d)"
email="$(printf '%%s' %s | base64 -d)"
username="$(printf '%%s' %s | base64 -d)"
password="$(printf '%%s' %s | base64 -d)"
user_id="$(printf '%%s' %s | base64 -d)"
channel_id="$(printf '%%s' %s | base64 -d)"
keep_artifacts=%s
timeout_seconds=%d

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" -X "$method" "$url")" || curl_status="$?"
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-browser-e2e-run-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
  if [ -n "${bot_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $mattermost_token" \
      "http://localhost:8065/api/v4/posts/$bot_post_id" >/dev/null || true
  fi
  if [ -n "${user_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $user_token" \
      "http://localhost:8065/api/v4/posts/$user_post_id" >/dev/null || true
  fi
  curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null 2>&1 || \
    curl --fail --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id" >/dev/null || true
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
}
trap cleanup EXIT

user_login_headers="$(mktemp)"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-probe-browser-e2e-run-user-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token" | jq -r '.id // empty')"
test -n "$bot_user_id"

post_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')"
user_post="$(api_request "post probe browser message" POST http://localhost:8065/api/v4/posts "$user_token" "$post_body")"
user_post_id="$(printf '%%s' "$user_post" | jq -r '.id')"
user_post_create_at="$(printf '%%s' "$user_post" | jq -r '.create_at')"
test -n "$user_post_id"

bot_post_id=""
for _ in $(seq 1 "$timeout_seconds"); do
  bot_post_id="$(api_request "wait for probe browser reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$user_post_create_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | .id' | head -1)"
  if [ -n "$bot_post_id" ]; then
    break
  fi
  sleep 1
done
if [ -z "$bot_post_id" ]; then
  echo "expected Mattermost bot reply for probe browser post $user_post_id" >&2
  exit 1
fi

bot_post_file="$(mktemp)"
api_request "fetch probe browser reply" GET "http://localhost:8065/api/v4/posts/$bot_post_id" "$admin_token" > "$bot_post_file"
task_run_id="$(blueclaw_request "find probe browser task" GET http://127.0.0.1:8080/admin/api/task |
  jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty')"
if [ -z "$task_run_id" ]; then
  echo "expected task for probe browser prompt" >&2
  exit 1
fi
task_detail_file="$(mktemp)"
blueclaw_request "probe browser task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
if ! jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "tool.browser.open.result" and (((.body // "{}") | fromjson? // {}) | .isError != true))' "$task_detail_file" >/dev/null; then
  echo "expected successful tool.browser.open.result for probe browser task $task_run_id" >&2
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi

jq -cn \
  --arg channel_id "$channel_id" \
  --arg user_post_id "$user_post_id" \
  --arg bot_post_id "$bot_post_id" \
  --arg task_run_id "$task_run_id" \
  --argjson keep "$keep_artifacts" \
  --slurpfile bot_post "$bot_post_file" \
  --slurpfile task_detail "$task_detail_file" \
  '{
    ok: true,
    kept: $keep,
    browserOpenVerified: true,
    channelID: $channel_id,
    userPostID: $user_post_id,
    botPostID: $bot_post_id,
    taskRunID: $task_run_id,
    botMessage: $bot_post[0].message,
    taskStatus: ((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskRun.status // null),
    taskEvents: (((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskEvents // []) | map({name, body: ((.body // "") | tostring | .[0:1200])}))
  }'
`,
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(prompt))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Email))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Username))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Password))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.UserID))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.ChannelID))),
		keepValue,
		timeoutSeconds,
	)
}
