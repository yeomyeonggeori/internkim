package setup

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anthropic-lab/internkim/auth"
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
// gcloudDefaultProject returns the gcloud config's current project id, or
// an empty string when none is set. Used to skip the "pick a project"
// prompt during `gws auth setup`.
func gcloudDefaultProject() string {
	out, err := exec.Command("gcloud", "config", "get-value", "project", "--quiet").Output()
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(out))
	if value == "(unset)" {
		return ""
	}
	return value
}

// enableWorkspaceAPIs pre-enables the Google Workspace API set that gws
// auth setup would otherwise prompt the user to pick interactively. If
// gws still shows the picker afterwards, the boxes are already ticked
// where it matters (all gws skills call only these APIs).
func enableWorkspaceAPIs(projectID string) {
	services := []string{
		"drive.googleapis.com",
		"sheets.googleapis.com",
		"gmail.googleapis.com",
		"calendar-json.googleapis.com",
		"docs.googleapis.com",
		"slides.googleapis.com",
		"tasks.googleapis.com",
		"people.googleapis.com",
		"forms.googleapis.com",
		"keep.googleapis.com",
		"meet.googleapis.com",
		"chat.googleapis.com",
		"script.googleapis.com",
	}
	args := append([]string{"services", "enable", "--project", projectID, "--quiet"}, services...)
	cmd := exec.Command("gcloud", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// ensureInternkimProject returns the GCP project id to use for this host,
// creating a dedicated "internkim-<device_id>" project the first time. The
// project id is deterministic per device, so re-runs always land on the
// same project — no cache files, no random suffixes, no orphan projects.
// Falls back to a random suffix only when device_id hasn't been assigned
// yet (very early in setup).
func ensureInternkimProject(context *Context) (string, error) {
	if existing := gcloudDefaultProject(); existing != "" {
		return existing, nil
	}

	projectID := internkimProjectID(context.Callbacks.LoadState)

	if err := createOrReuseGcpProject(projectID, context.T); err != nil {
		return "", err
	}
	exec.Command("gcloud", "config", "set", "project", projectID, "--quiet").Run()
	return projectID, nil
}

// internkimProjectID deterministically derives the per-device GCP project
// id from the device_id state. Falls back to a random suffix when no
// device_id exists yet.
func internkimProjectID(loadState func(key string) string) string {
	if loadState != nil {
		if deviceID := strings.TrimSpace(loadState("device_id")); deviceID != "" {
			return "internkim-" + deviceID
		}
	}
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "internkim-unknown"
	}
	return "internkim-" + hex.EncodeToString(randomBytes)
}

// createOrReuseGcpProject creates the project if absent; if it already
// exists and we own it, returns nil (reuse). Only errors on true failures.
func createOrReuseGcpProject(projectID string, translate func(korean, english string) string) error {
	describe := exec.Command("gcloud", "projects", "describe", projectID, "--quiet", "--format=value(projectId)")
	if out, err := describe.Output(); err == nil && strings.TrimSpace(string(out)) == projectID {
		return nil
	}
	fmt.Printf("  %s %s\n",
		translate("새 GCP 프로젝트 생성:", "Creating GCP project:"),
		projectID)
	create := exec.Command("gcloud", "projects", "create", projectID,
		"--name=Intern Kim", "--quiet")
	create.Stdout = os.Stdout
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return fmt.Errorf("gcloud projects create: %w", err)
	}
	return nil
}

// ensureGcloudAuthenticated verifies there's an active gcloud account and
// runs `gcloud auth login` interactively when there isn't. Both this step
// and `gws auth setup` need it, so we front-load it once.
func ensureGcloudAuthenticated(context *Context) error {
	out, _ := exec.Command("gcloud", "auth", "list",
		"--filter=status:ACTIVE", "--format=value(account)").Output()
	if strings.TrimSpace(string(out)) != "" {
		return nil
	}
	fmt.Println("  " + context.T(
		"gcloud 인증이 필요합니다. 브라우저 창이 열리면 Google 계정으로 로그인해주세요.",
		"gcloud not authenticated — opening browser window. Sign in with your Google account.",
	))
	login := exec.Command("gcloud", "auth", "login")
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return fmt.Errorf("gcloud auth login failed: %w", err)
	}
	return nil
}

// ensureGcloudOnPath makes `gcloud` discoverable for child processes even
// when the user installed the Google Cloud SDK tarball into ~ and has not
// yet opened a new shell. Idempotent; silently no-ops when gcloud is
// already on PATH or the common install dirs do not exist.
func ensureGcloudOnPath() {
	if _, err := exec.LookPath("gcloud"); err == nil {
		return
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "google-cloud-sdk", "bin"),
		"/opt/homebrew/share/google-cloud-sdk/bin",
		"/usr/local/share/google-cloud-sdk/bin",
	}
	pathValue := os.Getenv("PATH")
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "gcloud")); err == nil && !info.IsDir() {
			pathValue = candidate + string(os.PathListSeparator) + pathValue
			os.Setenv("PATH", pathValue)
			return
		}
	}
}

func exportGwsCredentials(context *Context) ([]byte, error) {
	if _, err := exec.LookPath("gws"); err != nil {
		return nil, errors.New(context.T(
			"gws CLI가 로컬 머신에 없습니다. 설치: brew install googleworkspace-cli",
			"gws CLI not found on local machine. Install: brew install googleworkspace-cli",
		))
	}

	ensureGcloudOnPath()

	exportCmd := exec.Command("gws", "auth", "export", "--unmasked")
	exportOut, exportErr := exportCmd.Output()
	if exportErr != nil || !strings.Contains(string(exportOut), `"refresh_token"`) {
		if _, err := exec.LookPath("gcloud"); err != nil {
			// gws auth setup needs gcloud to provision the GCP project.
			exec.Command("open", "https://cloud.google.com/sdk/docs/install").Start()
			return nil, errors.New(context.T(
				"gcloud CLI가 필요합니다. 설치: brew install --cask google-cloud-sdk\n"+
					"  (혹은 방금 연 페이지 참조: https://cloud.google.com/sdk/docs/install)\n"+
					"  설치 후 이 스텝을 다시 실행하세요.",
				"gcloud CLI is required. Install: brew install --cask google-cloud-sdk\n"+
					"  (or follow the page just opened: https://cloud.google.com/sdk/docs/install)\n"+
					"  Re-run this step once gcloud is installed.",
			))
		}
		if err := ensureGcloudAuthenticated(context); err != nil {
			return nil, err
		}
		projectID, err := ensureInternkimProject(context)
		if err != nil {
			return nil, err
		}
		enableWorkspaceAPIs(projectID)
		clientID, clientSecret, err := obtainUserOAuthClient(context, projectID)
		if err != nil {
			return nil, err
		}
		if err := gwsLogin(context, clientID, clientSecret); err != nil {
			return nil, err
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

// obtainUserOAuthClient launches the maintainer's Chrome (with a persistent
// profile that carries the Google sign-in cookie across runs) pointed at the
// Cloud Console credentials page, then drives the OAuth consent screen +
// Desktop client creation via chromedp. Returns the new (client_id,
// client_secret) pair — the only things Google blocks us from getting any
// other way for sensitive Workspace scopes on personal accounts.
func obtainUserOAuthClient(context *Context, projectID string) (string, string, error) {
	consoleURL := "https://console.cloud.google.com/apis/credentials?project=" + projectID + "&hl=en"
	if err := auth.LaunchChrome(consoleURL); err != nil {
		return "", "", fmt.Errorf("launch chrome: %w", err)
	}
	fmt.Println("  " + context.T(
		"Chrome 창이 열립니다. 처음이라면 Google 계정으로 로그인해주세요 (한 번만).",
		"Chrome window is opening. First time only: sign in with your Google account.",
	))
	return auth.EnsureUserOAuthClient(projectID)
}

// gwsLogin runs `gws auth login --full` with the user's own OAuth client
// injected via env vars. gws handles the loopback dance; the user clicks
// Allow once in the browser and the refresh_token lands in
// ~/.config/gws/credentials.json.
func gwsLogin(context *Context, clientID, clientSecret string) error {
	fmt.Println("  " + context.T(
		"브라우저에서 Allow만 클릭해주세요.",
		"Click Allow in the browser window.",
	))
	login := exec.Command("gws", "auth", "login", "--full")
	login.Env = append(os.Environ(),
		"GOOGLE_WORKSPACE_CLI_CLIENT_ID="+clientID,
		"GOOGLE_WORKSPACE_CLI_CLIENT_SECRET="+clientSecret,
	)
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return fmt.Errorf("gws auth login failed: %w", err)
	}
	return nil
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
