package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runStatus() {
	if errorValue := runStatusArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runStatusArguments(arguments []string) error {
	lang := "ko"
	if hasCommandArgument(arguments, "--en") {
		lang = "en"
	}
	m := newMsg(lang)
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	configuration := loadConfig()
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	target := resolveCommandTarget(arguments)
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	if hasCommandArgument(arguments, "--recover-ssh") {
		return runSSHRecoveryForTarget(m, configuration, sshpassBin, target, "restart-cloudflared-node-ssh", "", false)
	}
	return printStatusForCommandTarget(m, configuration, sshpassBin, target)
}

func printStatusForCommandTarget(m *msg, configuration config, sshpassBin string, target commandTarget) error {
	if strings.TrimSpace(target.host) == "" {
		target.host = findSavedSSHHostForStatus(target.stateDir)
	}
	if strings.TrimSpace(target.host) != "" {
		printCommandTargetEvidence(target)
		fmt.Printf("=== %s (%s: %s) ===\n\n", m.t("기기 상태", "Device Status"), target.boardType, target.host)
		printBoardStatus(m, target, newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host))
		return nil
	}
	if !target.useRemoteSSH && printPublicStatusForCommandTarget(m, target) {
		return nil
	}
	if connection, isRemote, errorValue := resolveRemoteSSHConnection(configuration, sshpassBin, target, false); errorValue == nil && connection != nil {
		target.host = connection.host
		target.useRemoteSSH = isRemote
		printCommandTargetEvidence(target)
		fmt.Printf("=== %s (%s: %s) ===\n\n", m.t("기기 상태", "Device Status"), target.boardType, target.host)
		printBoardStatus(m, target, connection)
		return nil
	} else if target.useRemoteSSH && errorValue != nil {
		if printPublicStatusForCommandTarget(m, target) {
			fmt.Printf("\n  %-20s ✗ %s\n", "SSH", errorValue)
			return nil
		}
		return errorValue
	}

	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue == nil && connection != nil {
		target.host = connection.host
		target.useRemoteSSH = isRemote
		printCommandTargetEvidence(target)
		fmt.Printf("=== %s (%s: %s) ===\n\n", m.t("기기 상태", "Device Status"), target.boardType, target.host)
		printBoardStatus(m, target, connection)
		return nil
	}
	if target.mode == commandTargetModeLab {
		return errors.New("lab target not found; run `internkim lab status` or pass --host <ip>")
	}

	if printPublicStatusForCommandTarget(m, target) {
		return nil
	}

	fmt.Printf("  %s\n", m.t(
		"기기를 찾을 수 없습니다.\n  - Board: Wi-Fi 연결 확인\n  - Lab: internkim lab vm-up",
		"Device not found.\n  - Board: Check Wi-Fi\n  - Lab: start with `internkim lab vm-up`",
	))
	return nil
}

func findSavedSSHHostForStatus(stateDirectory string) string {
	for _, host := range uniqueNonEmptyStrings([]string{
		loadState(stateDirectory, "board_ip"),
		loadState(stateDirectory, "board_wifi_ip"),
	}) {
		connection, errorValue := net.DialTimeout("tcp", host+":22", time.Second)
		if errorValue != nil {
			continue
		}
		connection.Close()
		return host
	}
	return ""
}

func printPublicStatusForCommandTarget(m *msg, target commandTarget) bool {
	if strings.TrimSpace(target.deviceURL) == "" {
		return false
	}
	printCommandTargetEvidence(target)
	fmt.Printf("=== %s (%s) ===\n\n", m.t("공개 URL 상태", "Public URL Status"), target.boardType)
	fmt.Printf("  %-20s %s\n", "SSH", "✗ "+m.t("로컬 SSH 미확인", "local SSH not found"))
	printPublicStatusSection(m, target)
	return true
}

func printPublicStatusSection(m *msg, target commandTarget) {
	if strings.TrimSpace(target.deviceURL) == "" {
		return
	}
	fmt.Println()
	for _, result := range publicEndpointStatuses(m, target.deviceURL) {
		fmt.Printf("  %-20s %s %s\n", result.label, result.marker, result.detail)
	}
}

type publicEndpointStatus struct {
	label  string
	marker string
	detail string
}

func publicEndpointStatuses(m *msg, deviceURL string) []publicEndpointStatus {
	return []publicEndpointStatus{
		publicEndpointStatusFor(m.t("Admin 공개 URL", "Admin public URL"), deviceURL, "/admin/api/health", `"status":"ok"`),
	}
}

func publicEndpointStatusFor(label string, deviceURL string, path string, expectedBodyFragment string) publicEndpointStatus {
	statusCode, responseBody, errorValue := fetchPublicEndpoint(deviceURL, path)
	if errorValue != nil {
		return publicEndpointStatus{label: label, marker: "✗", detail: errorValue.Error()}
	}
	if statusCode >= 200 && statusCode < 300 && strings.Contains(compactJSONSpaces(responseBody), expectedBodyFragment) {
		return publicEndpointStatus{label: label, marker: "✓", detail: fmt.Sprintf("HTTP %d", statusCode)}
	}
	if statusCode >= 300 && statusCode < 400 {
		return publicEndpointStatus{label: label, marker: "⏳", detail: fmt.Sprintf("HTTP %d redirect", statusCode)}
	}
	return publicEndpointStatus{label: label, marker: "✗", detail: fmt.Sprintf("HTTP %d", statusCode)}
}

func fetchPublicEndpoint(deviceURL string, path string) (int, string, error) {
	endpointURL, errorValue := publicEndpointURL(deviceURL, path)
	if errorValue != nil {
		return 0, "", errorValue
	}
	request, errorValue := http.NewRequest(http.MethodGet, endpointURL, nil)
	if errorValue != nil {
		return 0, "", errorValue
	}
	response, errorValue := statusHTTPClient.Do(request)
	if errorValue != nil {
		return 0, "", errorValue
	}
	defer response.Body.Close()
	responseBody, errorValue := io.ReadAll(io.LimitReader(response.Body, 4096))
	if errorValue != nil {
		return response.StatusCode, "", errorValue
	}
	return response.StatusCode, string(responseBody), nil
}

func publicEndpointURL(deviceURL string, path string) (string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(deviceURL))
	if errorValue != nil {
		return "", errorValue
	}
	parsedURL.Path = "/" + strings.TrimLeft(path, "/")
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String(), nil
}

func compactJSONSpaces(value string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(value), ""), " ", "")
}
