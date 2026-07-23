package cli

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const defaultMattermostVersion = "11.9.0"

const mattermostVersionEnvironmentVariable = "INTERNKIM_MATTERMOST_VERSION"

const mattermostDownloadAttemptCount = 3

const mattermostGuestTarballPath = "/tmp/mattermost.tar.gz"

const mattermostGuestTarballChecksumPath = "/tmp/mattermost.tar.gz.sha256"

type mattermostTarballDownloader func(downloadURL string, destinationPath string) error

func resolveMattermostVersion() string {
	if overrideVersion := strings.TrimSpace(os.Getenv(mattermostVersionEnvironmentVariable)); overrideVersion != "" {
		return overrideVersion
	}
	return defaultMattermostVersion
}

func mattermostDownloadURL(version string) string {
	return fmt.Sprintf("https://releases.mattermost.com/%s/mattermost-%s-linux-arm64.tar.gz", version, version)
}

func mattermostCacheDirectory() (string, error) {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil {
		return "", fmt.Errorf("resolve user home directory: %w", errorValue)
	}
	return filepath.Join(homeDirectory, ".cache", "internkim", "mattermost"), nil
}

func mattermostTarballCachePaths(version string) (tarballPath string, checksumPath string, err error) {
	cacheDirectory, errorValue := mattermostCacheDirectory()
	if errorValue != nil {
		return "", "", errorValue
	}
	tarballFileName := fmt.Sprintf("mattermost-%s-linux-arm64.tar.gz", version)
	tarballPath = filepath.Join(cacheDirectory, tarballFileName)
	return tarballPath, tarballPath + ".sha256", nil
}

func isValidGzipTarball(path string) bool {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return false
	}
	defer file.Close()
	gzipReader, errorValue := gzip.NewReader(file)
	if errorValue != nil {
		return false
	}
	defer gzipReader.Close()
	_, errorValue = io.Copy(io.Discard, gzipReader)
	return errorValue == nil
}

func sha256HexDigestForFile(path string) (string, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return "", errorValue
	}
	defer file.Close()
	hasher := sha256.New()
	if _, errorValue := io.Copy(hasher, file); errorValue != nil {
		return "", errorValue
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func isMattermostTarballCacheValid(tarballPath string, checksumPath string) bool {
	if _, errorValue := os.Stat(tarballPath); errorValue != nil {
		return false
	}
	expectedDigest, errorValue := os.ReadFile(checksumPath)
	if errorValue != nil {
		return false
	}
	actualDigest, errorValue := sha256HexDigestForFile(tarballPath)
	if errorValue != nil {
		return false
	}
	return strings.TrimSpace(string(expectedDigest)) == actualDigest
}

func downloadMattermostTarball(downloadURL string, destinationPath string) error {
	response, errorValue := http.Get(downloadURL)
	if errorValue != nil {
		return fmt.Errorf("download %s: %w", downloadURL, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", downloadURL, response.Status)
	}
	destinationFile, errorValue := os.Create(destinationPath)
	if errorValue != nil {
		return fmt.Errorf("create %s: %w", destinationPath, errorValue)
	}
	defer destinationFile.Close()
	if _, errorValue := io.Copy(destinationFile, response.Body); errorValue != nil {
		return fmt.Errorf("write %s: %w", destinationPath, errorValue)
	}
	return nil
}

func downloadMattermostTarballOnce(downloadURL string, tarballPath string, checksumPath string, download mattermostTarballDownloader) error {
	if errorValue := download(downloadURL, tarballPath); errorValue != nil {
		os.Remove(tarballPath)
		return errorValue
	}
	if !isValidGzipTarball(tarballPath) {
		os.Remove(tarballPath)
		return fmt.Errorf("downloaded tarball at %s failed gzip validation", tarballPath)
	}
	digest, errorValue := sha256HexDigestForFile(tarballPath)
	if errorValue != nil {
		os.Remove(tarballPath)
		return errorValue
	}
	if errorValue := os.WriteFile(checksumPath, []byte(digest), 0o644); errorValue != nil {
		os.Remove(tarballPath)
		return errorValue
	}
	return nil
}

func ensureMattermostTarballCached(version string, download mattermostTarballDownloader) (tarballPath string, checksumPath string, err error) {
	tarballPath, checksumPath, err = mattermostTarballCachePaths(version)
	if err != nil {
		return "", "", err
	}
	if isMattermostTarballCacheValid(tarballPath, checksumPath) {
		return tarballPath, checksumPath, nil
	}
	if err := os.MkdirAll(filepath.Dir(tarballPath), 0o755); err != nil {
		return "", "", fmt.Errorf("create Mattermost cache directory: %w", err)
	}
	downloadURL := mattermostDownloadURL(version)
	var lastError error
	for attemptIndex := 0; attemptIndex < mattermostDownloadAttemptCount; attemptIndex++ {
		lastError = downloadMattermostTarballOnce(downloadURL, tarballPath, checksumPath, download)
		if lastError == nil {
			return tarballPath, checksumPath, nil
		}
	}
	return "", "", fmt.Errorf("download Mattermost %s after %d attempts: %w", version, mattermostDownloadAttemptCount, lastError)
}

func pushMattermostTarballToGuest(ssh *sshClient, tarballPath string, checksumPath string) error {
	if errorValue := ssh.scp(tarballPath, mattermostGuestTarballPath); errorValue != nil {
		return fmt.Errorf("push Mattermost tarball to guest: %w", errorValue)
	}
	if errorValue := ssh.scp(checksumPath, mattermostGuestTarballChecksumPath); errorValue != nil {
		return fmt.Errorf("push Mattermost tarball checksum to guest: %w", errorValue)
	}
	return nil
}

func ensureMattermostTarballOnGuest(ssh *sshClient, version string) error {
	tarballPath, checksumPath, errorValue := ensureMattermostTarballCached(version, downloadMattermostTarball)
	if errorValue != nil {
		return errorValue
	}
	return pushMattermostTarballToGuest(ssh, tarballPath, checksumPath)
}

func mattermostGuestFallbackInstallScript(version string) string {
	return fmt.Sprintf(`
cd /tmp
cached_tarball_valid() {
  [ -s /tmp/mattermost.tar.gz ] || return 1
  [ -s /tmp/mattermost.tar.gz.sha256 ] || return 1
  [ "$(sha256sum /tmp/mattermost.tar.gz | awk '{print $1}')" = "$(cat /tmp/mattermost.tar.gz.sha256)" ]
}
if cached_tarball_valid; then
  echo "MMVER=cached"
  echo "download_ok"
else
  if [ -s /tmp/mattermost.tar.gz ]; then
    echo "MMSHA_EXPECTED=$(cat /tmp/mattermost.tar.gz.sha256 2>/dev/null)"
    echo "MMSHA_ACTUAL=$(sha256sum /tmp/mattermost.tar.gz | awk '{print $1}')"
  fi
  rm -f /tmp/mattermost.tar.gz /tmp/mattermost.tar.gz.sha256
  MMVER="%s"
  echo "MMVER=${MMVER}"
  URL="https://releases.mattermost.com/${MMVER}/mattermost-${MMVER}-linux-arm64.tar.gz"
  echo "Downloading Mattermost ${MMVER}..."
  downloadPath="/tmp/mattermost.tar.gz.download.$$"
  if curl -fsSL -o "$downloadPath" "$URL" 2>&1 | tail -1 && gzip -t "$downloadPath" 2>/dev/null; then
    sha256sum "$downloadPath" | awk '{print $1}' > "$downloadPath.sha256"
    mv "$downloadPath.sha256" /tmp/mattermost.tar.gz.sha256
    mv "$downloadPath" /tmp/mattermost.tar.gz
    echo "download_ok"
  else
    echo "MMSHA_ACTUAL=$(sha256sum "$downloadPath" 2>/dev/null | awk '{print $1}')"
    rm -f "$downloadPath" "$downloadPath.sha256"
    echo "download_failed"
  fi
fi`, version)
}
