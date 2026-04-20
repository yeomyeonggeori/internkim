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

	if err := stageAuthorizedUserCreds(context); err != nil {
		fmt.Println("  " + context.T("사용자 OAuth 자격증명 저장 실패 (계속)", "User OAuth creds save failed (continuing)"))
	}
	if err := exposeSAEmail(context); err != nil {
		fmt.Println("  " + context.T("SA 이메일 노출 실패 (계속)", "SA email expose failed (continuing)"))
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

	if err := stageAuthorizedUserCreds(context); err != nil {
		fmt.Println("  " + context.T("사용자 OAuth 자격증명 저장 실패 (계속)", "User OAuth creds save failed (continuing)"))
	}
	if err := exposeSAEmail(context); err != nil {
		fmt.Println("  " + context.T("SA 이메일 노출 실패 (계속)", "SA email expose failed (continuing)"))
	}
	return nil
}

// stageAuthorizedUserCreds writes the user's OAuth refresh_token to the target
// in ADC authorized_user format. The zeroclaw service reads this path via
// GOOGLE_APPLICATION_CREDENTIALS so the on-device gws CLI can act as the user
// (which has a real Drive quota, unlike the SA).
func stageAuthorizedUserCreds(context *Context) error {
	if context.Google == nil {
		return fmt.Errorf("google auth not performed")
	}
	authorizedUserJSON := context.Google.AuthorizedUserJSON()
	if authorizedUserJSON == "" {
		return fmt.Errorf("no refresh_token available — re-run with a fresh OAuth consent")
	}
	switch context.Backend {
	case BackendSSH:
		temporaryPath := filepath.Join(os.TempDir(), "google-user-creds.json")
		if err := os.WriteFile(temporaryPath, []byte(authorizedUserJSON), 0o600); err != nil {
			return err
		}
		defer os.Remove(temporaryPath)
		if err := context.SSH.SCP(temporaryPath, "/root/.internkim/secrets/google-user-creds.json"); err != nil {
			return err
		}
		context.SSH.Run(`chown root:zeroclaw /root/.internkim/secrets/google-user-creds.json
chmod 640 /root/.internkim/secrets/google-user-creds.json`)
		fmt.Println("  " + context.T("사용자 OAuth 자격증명 설치 완료", "User OAuth creds installed"))
	case BackendSD:
		if err := context.SD.WriteFile("secrets/google-user-creds.json", []byte(authorizedUserJSON), 0o644); err != nil {
			return err
		}
		fmt.Println("  " + context.T("사용자 OAuth 자격증명 SD에 스테이지함", "User OAuth creds staged to SD"))
	}
	return nil
}

// exposeSAEmail writes the bot's service-account email to the target so the
// agent can reference it when granting the bot writer access to files the
// user creates. Derived deterministically from the device_id.
func exposeSAEmail(context *Context) error {
	deviceID := context.Callbacks.LoadState("device_id")
	if deviceID == "" {
		return fmt.Errorf("device_id not set")
	}
	saEmail := fmt.Sprintf("internkim-%s@internkim-%s.iam.gserviceaccount.com", deviceID, deviceID)
	switch context.Backend {
	case BackendSSH:
		context.SSH.Run(fmt.Sprintf(
			`mkdir -p /root/.internkim/env
printf '%%s' %s > /root/.internkim/env/sa-email
chown root:zeroclaw /root/.internkim/env/sa-email
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
