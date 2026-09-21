package setup

import (
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBuzzMedia = Step{
	Name: "buzz-media",
	Deps: []string{"buzz-relay"},
	Title: func(context *Context) string {
		return context.T("Buzz 미디어 저장소 설치 중...", "Installing Buzz media store...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		return trimmedRun(context, "systemctl is-active "+blueclaw.BuzzMediaServiceName) == "active" &&
			trimmedRun(context, blueclaw.BuzzMediaHealthCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		connection := context.SSH

		release, errorValue := mediaReleaseForBoard(connection)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := installMediaBinary(connection, release); errorValue != nil {
			return errorValue
		}
		if errorValue := provisionMediaCredentials(connection); errorValue != nil {
			return errorValue
		}
		if errorValue := createMediaBucketDirectory(connection); errorValue != nil {
			return errorValue
		}
		if errorValue := startMediaService(connection); errorValue != nil {
			return errorValue
		}
		if errorValue := writeRelayMediaEnvironment(connection); errorValue != nil {
			return errorValue
		}
		connection.Run("systemctl restart " + blueclaw.BuzzRelayServiceName)

		fmt.Println("  " + context.T("Buzz 미디어 저장소 설치 완료", "Buzz media store installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func mediaReleaseForBoard(connection BoardConnection) (blueclaw.MediaServerRelease, error) {
	machine := strings.TrimSpace(connection.Run("uname -m"))
	release, isPublished := blueclaw.BuzzMediaReleaseFor(machine)
	if !isPublished {
		return release, fmt.Errorf("the board reports architecture %q and versitygw %s publishes %s, "+
			"so there is no media store to install and the messenger cannot hold attachments",
			machine, blueclaw.BuzzMediaVersion, strings.Join(blueclaw.BuzzMediaPublishedMachines(), " or "))
	}
	return release, nil
}

func installMediaBinary(connection BoardConnection, release blueclaw.MediaServerRelease) error {
	output := connection.Run(mediaBinaryInstallCommand(release))
	installed := strings.TrimSpace(connection.Run(mediaInstalledVersionCommand()))
	if strings.Contains(installed, blueclaw.BuzzMediaVersion) {
		return nil
	}
	return fmt.Errorf("%s reports %q after installing %s from %s, so the media store the messenger keeps attachments in is not on the board: %s",
		blueclaw.BuzzMediaBinaryPath, installed, release.AssetName, release.URL, tailOfRunOutput(output))
}

func mediaInstalledVersionCommand() string {
	return blueclaw.BuzzMediaBinaryPath + ` --version 2>/dev/null | head -1`
}

func mediaBinaryInstallCommand(release blueclaw.MediaServerRelease) string {
	return `if ` + mediaInstalledVersionCommand() + ` | grep -q '` + blueclaw.BuzzMediaVersion + `'; then exit 0; fi
set -e
curl -fsSL -o /tmp/versitygw.tar.gz '` + release.URL + `'
echo '` + release.SHA256 + `  /tmp/versitygw.tar.gz' | sha256sum -c -
tar -xzf /tmp/versitygw.tar.gz -C /tmp versitygw
install -m 0755 /tmp/versitygw ` + blueclaw.BuzzMediaBinaryPath + `
rm -f /tmp/versitygw.tar.gz /tmp/versitygw`
}

func provisionMediaCredentials(connection BoardConnection) error {
	output := connection.Run(mediaCredentialProvisionCommand())
	present := strings.TrimSpace(connection.Run(
		"test -s " + blueclaw.BuzzMediaEnvironmentFilePath + " && echo ok || echo no"))
	if present == "ok" {
		return nil
	}
	return fmt.Errorf("%s is empty or missing after provisioning the media store credentials, so neither the store nor the relay reading them can start: %s",
		blueclaw.BuzzMediaEnvironmentFilePath, tailOfRunOutput(output))
}

func mediaCredentialProvisionCommand() string {
	return `MEDIA_PASS=$(cat ` + blueclaw.BuzzMediaPasswordPath + ` 2>/dev/null || echo "")
if [ -z "$MEDIA_PASS" ]; then
  MEDIA_PASS=$(head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
  printf '%s' "$MEDIA_PASS" > ` + blueclaw.BuzzMediaPasswordPath + `
  chmod 600 ` + blueclaw.BuzzMediaPasswordPath + `
fi
{
  printf 'ROOT_ACCESS_KEY_ID=%s\n' '` + blueclaw.BuzzMediaAccessKey + `'
  printf 'ROOT_SECRET_ACCESS_KEY=%s\n' "$MEDIA_PASS"
} > ` + blueclaw.BuzzMediaEnvironmentFilePath + `
chmod 600 ` + blueclaw.BuzzMediaEnvironmentFilePath
}

// createMediaBucketDirectory is what `mc mb` used to do. The posix backend
// serves every directory under the gateway root as a bucket, so the bucket is
// created and owned here instead of by an S3 client. 0750 root:root matches
// what the MinIO data directory carried, and keeps attachments out of reach of
// the unprivileged task users sharing the board; the relay never touches these
// files, it speaks S3 to 127.0.0.1.
func createMediaBucketDirectory(connection BoardConnection) error {
	bucketPath := blueclaw.BuzzMediaBucketPath()
	output := connection.Run(
		"install -d -o root -g root -m 750 " + blueclaw.BuzzMediaRootPath + " " + bucketPath)
	present := strings.TrimSpace(connection.Run("test -d " + bucketPath + " && echo ok || echo no"))
	if present == "ok" {
		return nil
	}
	return fmt.Errorf("%s does not exist after creating it, so the bucket the relay writes attachments to is missing: %s",
		bucketPath, tailOfRunOutput(output))
}

func startMediaService(connection BoardConnection) error {
	output := connection.Run(mediaUnitInstallCommand())
	state := strings.TrimSpace(connection.Run("systemctl is-active " + blueclaw.BuzzMediaServiceName))
	health := strings.TrimSpace(connection.Run(blueclaw.BuzzMediaHealthCheckCommand()))
	if state == "active" && health == "ok" {
		return nil
	}
	return fmt.Errorf("%s is %s and its health endpoint answers %s after installing the unit, so attachments have nowhere to go: %s",
		blueclaw.BuzzMediaServiceName, state, health, tailOfRunOutput(output))
}

func mediaUnitInstallCommand() string {
	return `cat > ` + blueclaw.BuzzMediaServicePath + ` <<'MEDIAUNITEOF'
` + blueclaw.BuzzMediaServiceUnit() + `MEDIAUNITEOF
systemctl daemon-reload
systemctl enable ` + blueclaw.BuzzMediaServiceName + `
systemctl restart ` + blueclaw.BuzzMediaServiceName + `
for attempt in $(seq 1 30); do
  curl -fsS --max-time 3 http://` + blueclaw.BuzzMediaAddress + blueclaw.BuzzMediaHealthPath + ` >/dev/null 2>&1 && break
  sleep 1
done
journalctl -u ` + blueclaw.BuzzMediaServiceName + ` --no-pager -n 20 2>/dev/null || true`
}

func writeRelayMediaEnvironment(connection BoardConnection) error {
	output := connection.Run(buzzRelayS3EnvironmentCommand())
	named := strings.TrimSpace(connection.Run(
		"grep -c '^BUZZ_S3_SECRET_KEY=.' " + blueclaw.BuzzRelayS3EnvironmentFilePath + " 2>/dev/null || echo 0"))
	if named == "1" {
		return nil
	}
	return fmt.Errorf("%s does not carry BUZZ_S3_SECRET_KEY after writing it, so the relay starts without the media store it keeps attachments in: %s",
		blueclaw.BuzzRelayS3EnvironmentFilePath, tailOfRunOutput(output))
}

func buzzRelayS3EnvironmentCommand() string {
	return `MEDIA_PASS=$(cat ` + blueclaw.BuzzMediaPasswordPath + `)
{
  printf 'BUZZ_S3_ENDPOINT=http://%s\n' '` + blueclaw.BuzzMediaAddress + `'
  printf 'BUZZ_S3_ACCESS_KEY=%s\n' '` + blueclaw.BuzzMediaAccessKey + `'
  printf 'BUZZ_S3_SECRET_KEY=%s\n' "$MEDIA_PASS"
  printf 'BUZZ_S3_BUCKET=%s\n' '` + blueclaw.BuzzMediaBucket + `'
} > ` + blueclaw.BuzzRelayS3EnvironmentFilePath + `
chmod 600 ` + blueclaw.BuzzRelayS3EnvironmentFilePath
}

func tailOfRunOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	return strings.TrimSpace(strings.Join(lines, " | "))
}
