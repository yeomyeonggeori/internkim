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
		return context.T("Google Workspace 서비스 계정 설정...", "Setting up Google Workspace service account...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSD:
			for _, name := range []string{"secrets/google-sa.json", "sa-email", "secrets/gas-webhook-url"} {
				if _, statError := os.Stat(filepath.Join(context.SD.RootPath(), name)); statError != nil {
					return false
				}
			}
			return true
		default:
			output := strings.TrimSpace(context.SSH.Run(
				`test -f /root/.internkim/secrets/google-sa.json && ` +
					`test -f /root/.internkim/env/sa-email && ` +
					`test -f /root/.internkim/secrets/gas-webhook-url && echo yes || echo no`,
			))
			return output == "yes"
		}
	},
	Run:   runGoogleSSH,
	RunSD: runGoogleSD,
}

func fetchServiceAccountKey(context *Context) ([]byte, string, error) {
	if context.Google == nil || context.Google.AccessToken == "" {
		if context.Callbacks.GoogleAuth == nil {
			return nil, "", errors.New("google auth callback missing")
		}
		googleAuth, err := context.Callbacks.GoogleAuth()
		if err != nil {
			return nil, "", fmt.Errorf("oauth: %w", err)
		}
		context.Google = googleAuth
	}
	deviceID := context.Callbacks.LoadState("device_id")
	if deviceID == "" {
		return nil, "", errors.New("device_id not set — run earlier steps first")
	}
	serviceAccountKey, err := context.Callbacks.CreateGoogleSA(deviceID, context.Google.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("SA key: %w", err)
	}
	return []byte(serviceAccountKey), deviceID, nil
}

func runGoogleSSH(context *Context) error {
	keyBytes, deviceID, err := fetchServiceAccountKey(context)
	if err != nil {
		return err
	}
	temporaryPath := filepath.Join(os.TempDir(), "gsa-"+deviceID+".json")
	if err := os.WriteFile(temporaryPath, keyBytes, 0o600); err != nil {
		return err
	}
	defer os.Remove(temporaryPath)
	if err := context.SSH.SCP(temporaryPath, "/root/.internkim/secrets/google-sa.json"); err != nil {
		return fmt.Errorf("scp sa.json: %w", err)
	}
	context.SSH.Run(`chown root:blueclaw /root/.internkim/secrets/google-sa.json
chmod 640 /root/.internkim/secrets/google-sa.json`)
	fmt.Println("  " + context.T("서비스 계정 생성 완료 (live)", "Service account created (live)"))

	if err := exposeSAEmail(context); err != nil {
		fmt.Println("  " + context.T("SA 이메일 노출 실패 (계속)", "SA email expose failed (continuing)"))
	}
	if err := stageGasWebhookURL(context); err != nil {
		return fmt.Errorf("gas webhook: %w", err)
	}
	return nil
}

func runGoogleSD(context *Context) error {
	keyBytes, _, err := fetchServiceAccountKey(context)
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile("secrets/google-sa.json", keyBytes, 0o644); err != nil {
		return fmt.Errorf("write sa.json to SD: %w", err)
	}
	fmt.Printf("  %s (%s)\n",
		context.T("SA 키를 SD에 스테이지함 — 다음 부팅에 적용됨", "SA key staged to SD — applies on next boot"),
		filepath.Join(context.SD.RootPath(), "secrets", "google-sa.json"))

	if err := exposeSAEmail(context); err != nil {
		fmt.Println("  " + context.T("SA 이메일 노출 실패 (계속)", "SA email expose failed (continuing)"))
	}
	if err := stageGasWebhookURL(context); err != nil {
		return fmt.Errorf("gas webhook: %w", err)
	}
	return nil
}

func exposeSAEmail(context *Context) error {
	deviceID := context.Callbacks.LoadState("device_id")
	if deviceID == "" {
		return fmt.Errorf("device_id not set")
	}
	saEmail := fmt.Sprintf(
		"internkim-%s@internkim-%s.iam.gserviceaccount.com",
		googleResourceSuffix(deviceID),
		googleResourceSuffix(deviceID),
	)
	switch context.Backend {
	case BackendSSH:
		context.SSH.Run(fmt.Sprintf(
			`mkdir -p /root/.internkim/env
printf '%%s' %s > /root/.internkim/env/sa-email
chown root:blueclaw /root/.internkim/env/sa-email
chmod 640 /root/.internkim/env/sa-email`,
			shellQuote(saEmail),
		))
	case BackendSD:
		if err := context.SD.WriteFile("sa-email", []byte(saEmail), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func googleResourceSuffix(deviceID string) string {
	normalizedDeviceID := strings.ToLower(strings.TrimSpace(deviceID))
	if len(normalizedDeviceID) > 20 {
		return normalizedDeviceID[:20]
	}
	return normalizedDeviceID
}

func stageGasWebhookURL(context *Context) error {
	if context.Callbacks.GetGasWebhookURL == nil {
		return errors.New("GAS webhook URL callback missing")
	}
	accessToken := ""
	if context.Google != nil {
		accessToken = context.Google.AccessToken
	}
	webhookURL, err := context.Callbacks.GetGasWebhookURL(accessToken)
	if err != nil {
		return err
	}
	switch context.Backend {
	case BackendSSH:
		temporaryPath := filepath.Join(os.TempDir(), "gas-webhook-url")
		if err := os.WriteFile(temporaryPath, []byte(webhookURL+"\n"), 0o600); err != nil {
			return err
		}
		defer os.Remove(temporaryPath)
		if err := context.SSH.SCP(temporaryPath, "/root/.internkim/secrets/gas-webhook-url"); err != nil {
			return err
		}
		context.SSH.Run(`chown root:blueclaw /root/.internkim/secrets/gas-webhook-url
chmod 640 /root/.internkim/secrets/gas-webhook-url`)
		fmt.Println("  " + context.T("GAS 웹훅 URL 설치 완료", "GAS webhook URL installed"))
	case BackendSD:
		if err := context.SD.WriteFile("secrets/gas-webhook-url", []byte(webhookURL+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Println("  " + context.T("GAS 웹훅 URL SD에 스테이지함", "GAS webhook URL staged to SD"))
	}
	return nil
}
