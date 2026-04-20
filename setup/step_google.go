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
			_, statError := os.Stat(filepath.Join(context.SD.RootPath(), "secrets", "google-sa.json"))
			return statError == nil
		default:
			output := strings.TrimSpace(context.SSH.Run(
				"test -f /root/.internkim/secrets/google-sa.json && echo yes || echo no",
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
	context.SSH.Run(`chown root:zeroclaw /root/.internkim/secrets/google-sa.json
chmod 640 /root/.internkim/secrets/google-sa.json`)
	fmt.Println("  " + context.T("서비스 계정 생성 완료 (live)", "Service account created (live)"))
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
	return nil
}
