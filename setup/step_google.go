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
		out := strings.TrimSpace(ctx.SSH.Run(
			"test -f /root/.internkim/secrets/google-sa.json && echo yes || echo no",
		))
		return out == "yes"
	},
	Run: func(ctx *Context) error {
		if ctx.Google == nil || ctx.Google.AccessToken == "" {
			// Try to obtain it now.
			if ctx.Cb.GoogleAuth == nil {
				return errors.New("google auth callback missing")
			}
			g, err := ctx.Cb.GoogleAuth()
			if err != nil {
				return fmt.Errorf("oauth: %w", err)
			}
			ctx.Google = g
		}

		deviceID := ctx.Cb.LoadState("device_id")
		if deviceID == "" {
			return errors.New("device_id not set — run earlier steps first")
		}

		// CreateGoogleSA internally resolves the project and enables required
		// APIs (drive/docs/sheets/slides/gmail/calendar/tasks/people/chat/
		// forms/script/classroom/meet/keep) before issuing the key.
		saKey, err := ctx.Cb.CreateGoogleSA(deviceID, ctx.Google.AccessToken)
		if err != nil {
			return fmt.Errorf("SA key: %w", err)
		}

		tmp := filepath.Join(os.TempDir(), "gsa-"+deviceID+".json")
		if err := os.WriteFile(tmp, []byte(saKey), 0o600); err != nil {
			return err
		}
		defer os.Remove(tmp)

		if err := ctx.SSH.SCP(tmp, "/root/.internkim/secrets/google-sa.json"); err != nil {
			return fmt.Errorf("scp sa.json: %w", err)
		}
		ctx.SSH.Run(`chown gws /root/.internkim/secrets/google-sa.json
chmod 640 /root/.internkim/secrets/google-sa.json`)
		fmt.Println("  " + ctx.T("서비스 계정 생성 완료", "Service account created"))
		return nil
	},
}
