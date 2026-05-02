package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var StepGoogle = Step{
	Name: "google",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("Google Workspace 자격증명 설치...", "Installing Google Workspace credentials...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSD:
			_, statError := os.Stat(filepath.Join(context.SD.RootPath(), "secrets", "gas-webhook-url"))
			return statError == nil
		default:
			output := strings.TrimSpace(context.SSH.Run(`test -f /root/.internkim/secrets/gas-webhook-url && echo yes || echo no`))
			return output == "yes"
		}
	},
	Run:   runGoogleSSH,
	RunSD: runGoogleSD,
}

func runGoogleSSH(context *Context) error {
	webhookURL, errorValue := googleWorkspaceWebhookURL(context)
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := filepath.Join(os.TempDir(), "gas-webhook-url")
	if errorValue := os.WriteFile(temporaryPath, []byte(webhookURL+"\n"), 0600); errorValue != nil {
		return errorValue
	}
	defer os.Remove(temporaryPath)
	if errorValue := context.SSH.SCP(temporaryPath, "/root/.internkim/secrets/gas-webhook-url"); errorValue != nil {
		return errorValue
	}
	context.SSH.Run(`chown root:root /root/.internkim/secrets/gas-webhook-url
chmod 600 /root/.internkim/secrets/gas-webhook-url`)
	fmt.Println("  " + context.T("Google Workspace 웹훅 설치 완료", "Google Workspace webhook installed"))
	return nil
}

func runGoogleSD(context *Context) error {
	webhookURL, errorValue := googleWorkspaceWebhookURL(context)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := context.SD.WriteFile("secrets/gas-webhook-url", []byte(webhookURL+"\n"), 0600); errorValue != nil {
		return errorValue
	}
	fmt.Println("  " + context.T("Google Workspace 웹훅을 SD에 스테이지함", "Google Workspace webhook staged to SD"))
	return nil
}

func googleWorkspaceWebhookURL(context *Context) (string, error) {
	if context.Callbacks.GetGasWebhookURL == nil {
		return "", errors.New("Google Workspace credential callback missing")
	}
	webhookURL, errorValue := context.Callbacks.GetGasWebhookURL("")
	if errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(webhookURL) == "" {
		return "", errors.New("Google Workspace webhook URL is empty")
	}
	return strings.TrimSpace(webhookURL), nil
}
