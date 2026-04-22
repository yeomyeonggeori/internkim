package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// StepGoogle obtains Google Workspace credentials on the maintainer's
// Mac via the gws CLI (GCP project + OAuth client + refresh token) and
// stages them to the target so the on-device agent can call every
// Workspace API as the user directly.
//
// The flow is:
//  1. Ensure `gws` CLI is installed locally.
//  2. Run `gws auth setup` + `gws auth login` the first time; subsequent
//     runs re-use the existing credential.
//  3. `gws auth export --unmasked` produces a portable 4-field JSON.
//  4. The JSON ships to /home/zeroclaw/.config/gws/credentials.json
//     and zeroclaw.service points GOOGLE_WORKSPACE_CLI_CREDENTIALS_FILE
//     at it.
var StepGoogle = Step{
	Name: "google",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("Google Workspace 인증 준비...", "Preparing Google Workspace auth...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSD:
			_, err := os.Stat(filepath.Join(context.SD.RootPath(), "secrets/gws-credentials.json"))
			return err == nil
		default:
			output := strings.TrimSpace(context.SSH.Run(
				`test -f /home/zeroclaw/.config/gws/credentials.json && echo yes || echo no`,
			))
			return output == "yes"
		}
	},
	Run:   runGoogleSSH,
	RunSD: runGoogleSD,
}

// exportGwsCredentials ensures the maintainer's local gws state is
// authenticated, then returns the portable credentials JSON as bytes.
// Calls `gws auth setup` the first time (interactive, opens browser);
// subsequent calls just re-export the existing refresh token.
func exportGwsCredentials(context *Context) ([]byte, error) {
	if _, err := exec.LookPath("gws"); err != nil {
		return nil, errors.New(context.T(
			"gws CLI가 로컬 머신에 없습니다. 설치: brew install googleworkspace/tap/gws (또는 https://github.com/googleworkspace/cli 참조)",
			"gws CLI not found on local machine. Install: brew install googleworkspace/tap/gws (or see https://github.com/googleworkspace/cli)",
		))
	}

	exportCmd := exec.Command("gws", "auth", "export", "--unmasked")
	exportOut, exportErr := exportCmd.Output()
	if exportErr != nil || !strings.Contains(string(exportOut), `"refresh_token"`) {
		fmt.Println("  " + context.T(
			"기존 gws 자격증명을 찾지 못해 새로 발급합니다. 브라우저 창에서 Google 계정으로 로그인 + 허용 눌러주세요.",
			"No existing gws credentials found — starting setup. Log in with your Google account and click Allow in the browser.",
		))
		setup := exec.Command("gws", "auth", "setup")
		setup.Stdin = os.Stdin
		setup.Stdout = os.Stdout
		setup.Stderr = os.Stderr
		if err := setup.Run(); err != nil {
			return nil, fmt.Errorf("gws auth setup failed: %w", err)
		}
		exportCmd = exec.Command("gws", "auth", "export", "--unmasked")
		exportOut, exportErr = exportCmd.Output()
		if exportErr != nil {
			return nil, fmt.Errorf("gws auth export failed: %w", exportErr)
		}
		if !strings.Contains(string(exportOut), `"refresh_token"`) {
			return nil, errors.New("gws auth export output missing refresh_token")
		}
	}
	return exportOut, nil
}

func runGoogleSSH(context *Context) error {
	credentials, err := exportGwsCredentials(context)
	if err != nil {
		return err
	}
	temporaryPath := filepath.Join(os.TempDir(), "internkim-gws-credentials.json")
	if err := os.WriteFile(temporaryPath, credentials, 0o600); err != nil {
		return err
	}
	defer os.Remove(temporaryPath)

	context.SSH.Run(`mkdir -p /home/zeroclaw/.config/gws
chown -R zeroclaw:zeroclaw /home/zeroclaw/.config`)
	if err := context.SSH.SCP(temporaryPath, "/home/zeroclaw/.config/gws/credentials.json"); err != nil {
		return fmt.Errorf("scp credentials: %w", err)
	}
	context.SSH.Run(`chown zeroclaw:zeroclaw /home/zeroclaw/.config/gws/credentials.json
chmod 600 /home/zeroclaw/.config/gws/credentials.json`)
	fmt.Println("  " + context.T(
		"gws 자격증명 설치 완료 (유저 identity로 모든 Workspace API 호출 가능)",
		"gws credentials installed — agent can call every Workspace API as the user",
	))
	return nil
}

func runGoogleSD(context *Context) error {
	credentials, err := exportGwsCredentials(context)
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile("secrets/gws-credentials.json", credentials, 0o600); err != nil {
		return fmt.Errorf("write gws credentials to SD: %w", err)
	}
	fmt.Printf("  %s (%s)\n",
		context.T("gws 자격증명을 SD에 스테이지함", "gws credentials staged to SD"),
		filepath.Join(context.SD.RootPath(), "secrets", "gws-credentials.json"))
	return nil
}
