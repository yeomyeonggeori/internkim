package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	boardDefaultUser = "internkim"
	boardDefaultHost = "internkim.local"
)

type commandTarget struct {
	host        string
	sshUser     string
	sshPassword string
	deviceURL   string
	fleetID     string
	fleetSecret string
}

func resolveCommandTarget(arguments []string) commandTarget {
	return commandTarget{
		host:        firstNonEmptyString(commandArgumentValue(arguments, "--host", ""), os.Getenv("INTERNKIM_SSH_HOSTNAME"), boardDefaultHost),
		sshUser:     commandArgumentValue(arguments, "--user", boardDefaultUser),
		sshPassword: firstNonEmptyString(commandArgumentValue(arguments, "--password", ""), os.Getenv("INTERNKIM_CONSOLE_PASSWORD")),
		deviceURL:   normalizeDeviceURL(firstNonEmptyString(commandArgumentValue(arguments, "--device-url", ""), os.Getenv("INTERNKIM_DEVICE_URL"))),
		fleetID:     strings.ToLower(strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_ID"))),
		fleetSecret: strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_SECRET")),
	}
}

func (target commandTarget) sshConnection() *sshClient {
	return newSSH(target.sshUser, target.sshPassword, target.host)
}

func (target commandTarget) fleetIdentity() (string, string, error) {
	if target.fleetID == "" || target.fleetSecret == "" {
		return "", "", errors.New("the vault names no INTERNKIM_FLEET_ID and INTERNKIM_FLEET_SECRET for this device; run it as `internkim @legacy …`")
	}
	return target.fleetID, target.fleetSecret, nil
}

func printCommandTargetEvidence(target commandTarget) {
	fmt.Printf("Host: %s\n", target.host)
	if target.fleetID != "" {
		fmt.Printf("Fleet: %s\n", target.fleetID)
	}
	if target.deviceURL != "" {
		fmt.Printf("URL: %s\n", target.deviceURL)
	}
}

func normalizeDeviceURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}

func targetFlagArguments(host string, user string, password string) []string {
	arguments := []string{}
	for _, flag := range [][2]string{{"--host", host}, {"--user", user}, {"--password", password}} {
		if strings.TrimSpace(flag[1]) != "" {
			arguments = append(arguments, flag[0], flag[1])
		}
	}
	return arguments
}
