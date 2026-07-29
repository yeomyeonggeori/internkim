package setup

import (
	"fmt"

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
		return trimmedRun(context, "systemctl is-active "+blueclaw.MinioServiceName) == "active" &&
			trimmedRun(context, blueclaw.MinioHealthCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		connection := context.SSH

		connection.Run(minioBinariesInstallCommand())
		connection.Run(minioCredentialProvisionCommand())
		connection.Run(minioUnitInstallCommand())
		connection.Run(minioBucketProvisionCommand())
		connection.Run(buzzRelayS3EnvironmentCommand())
		connection.Run("systemctl restart " + blueclaw.BuzzRelayServiceName)

		fmt.Println("  " + context.T("Buzz 미디어 저장소 설치 완료", "Buzz media store installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func minioBinariesInstallCommand() string {
	return `if [ ! -x ` + blueclaw.MinioBinaryPath + ` ]; then
  curl -fsSL -o ` + blueclaw.MinioBinaryPath + ` ` + blueclaw.MinioBinaryURL + `
  chmod +x ` + blueclaw.MinioBinaryPath + `
fi
if [ ! -x ` + blueclaw.McBinaryPath + ` ]; then
  curl -fsSL -o ` + blueclaw.McBinaryPath + ` ` + blueclaw.McBinaryURL + `
  chmod +x ` + blueclaw.McBinaryPath + `
fi
install -d -o root -g root -m 750 ` + blueclaw.MinioDataPath
}

func minioCredentialProvisionCommand() string {
	return `MINIO_PASS=$(cat ` + blueclaw.MinioPasswordPath + ` 2>/dev/null || echo "")
if [ -z "$MINIO_PASS" ]; then
  MINIO_PASS=$(head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
  printf '%s' "$MINIO_PASS" > ` + blueclaw.MinioPasswordPath + `
  chmod 600 ` + blueclaw.MinioPasswordPath + `
fi
{
  printf 'MINIO_ROOT_USER=%s\n' '` + blueclaw.BuzzMediaAccessKey + `'
  printf 'MINIO_ROOT_PASSWORD=%s\n' "$MINIO_PASS"
} > ` + blueclaw.MinioEnvironmentFilePath + `
chmod 600 ` + blueclaw.MinioEnvironmentFilePath
}

func minioUnitInstallCommand() string {
	return `cat > ` + blueclaw.MinioServicePath + ` <<'MINIOUNITEOF'
` + blueclaw.MinioServiceUnit() + `MINIOUNITEOF
systemctl daemon-reload
systemctl enable ` + blueclaw.MinioServiceName + `
systemctl restart ` + blueclaw.MinioServiceName + `
for attempt in $(seq 1 30); do
  curl -fsS --max-time 3 http://` + blueclaw.MinioAddress + `/minio/health/ready >/dev/null 2>&1 && break
  sleep 1
done`
}

func minioBucketProvisionCommand() string {
	return `MINIO_PASS=$(cat ` + blueclaw.MinioPasswordPath + `)
` + blueclaw.McBinaryPath + ` alias set buzzlocal http://` + blueclaw.MinioAddress + ` ` + blueclaw.BuzzMediaAccessKey + ` "$MINIO_PASS" >/dev/null 2>&1
` + blueclaw.McBinaryPath + ` mb --ignore-existing buzzlocal/` + blueclaw.BuzzMediaBucket + ` >/dev/null 2>&1`
}

func buzzRelayS3EnvironmentCommand() string {
	return `MINIO_PASS=$(cat ` + blueclaw.MinioPasswordPath + `)
{
  printf 'BUZZ_S3_ENDPOINT=http://%s\n' '` + blueclaw.MinioAddress + `'
  printf 'BUZZ_S3_ACCESS_KEY=%s\n' '` + blueclaw.BuzzMediaAccessKey + `'
  printf 'BUZZ_S3_SECRET_KEY=%s\n' "$MINIO_PASS"
  printf 'BUZZ_S3_BUCKET=%s\n' '` + blueclaw.BuzzMediaBucket + `'
} > ` + blueclaw.BuzzRelayS3EnvironmentFilePath + `
chmod 600 ` + blueclaw.BuzzRelayS3EnvironmentFilePath
}
