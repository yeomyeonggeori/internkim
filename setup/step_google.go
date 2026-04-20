package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StepGoogle creates/refreshes the GCP service account and enables the
// Google Workspace APIs that gws-cli skills exercise.
var StepGoogle = Step{
	Name: "google",
	Deps: []string{"binaries"}, // gws binary must exist on board to use SA
	Title: func(ctx *Context) string {
		return ctx.T("Google Workspace 서비스 계정 설정...", "Setting up Google Workspace service account...")
	},
	IsSatisfied: func(ctx *Context) bool {
		switch ctx.Backend {
		case BackendSD:
			_, err := os.Stat(filepath.Join(ctx.SD.RootPath(), "secrets", "google-sa.json"))
			return err == nil
		default:
			out := strings.TrimSpace(ctx.SSH.Run(
				"test -f /root/.internkim/secrets/google-sa.json && echo yes || echo no",
			))
			return out == "yes"
		}
	},
	Run:   stepGoogleSSH,
	RunSD: stepGoogleSD,
}

// fetchSAKey handles the parts shared by both backends — OAuth, project
// resolution, API enable, SA key issuance. Returns the JSON key bytes.
func fetchSAKey(ctx *Context) ([]byte, string, error) {
	if ctx.Google == nil || ctx.Google.AccessToken == "" {
		if ctx.Cb.GoogleAuth == nil {
			return nil, "", errors.New("google auth callback missing")
		}
		g, err := ctx.Cb.GoogleAuth()
		if err != nil {
			return nil, "", fmt.Errorf("oauth: %w", err)
		}
		ctx.Google = g
	}
	deviceID := ctx.Cb.LoadState("device_id")
	if deviceID == "" {
		return nil, "", errors.New("device_id not set — run earlier steps first")
	}
	// CreateGoogleSA internally resolves the project and enables the 14
	// required APIs before issuing the key.
	saKey, err := ctx.Cb.CreateGoogleSA(deviceID, ctx.Google.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("SA key: %w", err)
	}
	return []byte(saKey), deviceID, nil
}

func stepGoogleSSH(ctx *Context) error {
	key, deviceID, err := fetchSAKey(ctx)
	if err != nil {
		return err
	}
	tmp := filepath.Join(os.TempDir(), "gsa-"+deviceID+".json")
	if err := os.WriteFile(tmp, key, 0o600); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := ctx.SSH.SCP(tmp, "/root/.internkim/secrets/google-sa.json"); err != nil {
		return fmt.Errorf("scp sa.json: %w", err)
	}
	ctx.SSH.Run(`chown gws /root/.internkim/secrets/google-sa.json
chmod 640 /root/.internkim/secrets/google-sa.json`)
	fmt.Println("  " + ctx.T("서비스 계정 생성 완료 (live)", "Service account created (live)"))
	return nil
}

func stepGoogleSD(ctx *Context) error {
	key, _, err := fetchSAKey(ctx)
	if err != nil {
		return err
	}
	if err := ctx.SD.WriteFile("secrets/google-sa.json", key, 0o644); err != nil {
		return fmt.Errorf("write sa.json to SD: %w", err)
	}
	fmt.Printf("  %s (%s)\n",
		ctx.T("SA 키를 SD에 스테이지함 — 다음 부팅에 적용됨", "SA key staged to SD — applies on next boot"),
		filepath.Join(ctx.SD.RootPath(), "secrets", "google-sa.json"))
	return nil
}
