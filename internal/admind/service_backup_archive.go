package admind

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"strings"
	"time"
)

type BackupManifest struct {
	FormatVersion  int               `json:"formatVersion"`
	FleetID        string            `json:"fleetID"`
	CreatedAt      time.Time         `json:"createdAt"`
	Components     []string          `json:"components"`
	Checksums      map[string]string `json:"checksums"`
	InternKim      map[string]string `json:"internKim"`
	Blueclaw       map[string]any    `json:"blueclaw,omitempty"`
	BlueclawDump   bool              `json:"blueclawDump"`
}

func backupIncludedPaths() []string {
	return []string{
		"/root/.internkim/env",
		"/root/.internkim/config",
		"/root/.internkim/secrets",
		"/root/.internkim/state",
		"/root/.internkim/sites",
		"/root/.blueclaw/config",
		"/root/.blueclaw/workspace",
		"/var/lib/blueclaw/workspace.ext4",
		"/etc/cloudflared",
	}
}

func addManifestToTar(tarWriter *tar.Writer, manifest *BackupManifest) error {
	document, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return addBytesToTar(tarWriter, "manifest.json", document)
}

func addPathToTar(tarWriter *tar.Writer, includedPath string, manifest *BackupManifest) error {
	fileInfo, errorValue := os.Stat(includedPath)
	if errorValue != nil {
		return errorValue
	}
	if fileInfo.IsDir() {
		return filepath.Walk(includedPath, func(path string, information os.FileInfo, walkError error) error {
			if walkError != nil || information.IsDir() {
				return walkError
			}
			return addFileToTar(tarWriter, path, tarName(path), manifest)
		})
	}
	return addFileToTar(tarWriter, includedPath, tarName(includedPath), manifest)
}

func addNamedFileToTar(tarWriter *tar.Writer, filePath string, tarPath string, manifest *BackupManifest) error {
	return addFileToTar(tarWriter, filePath, tarPath, manifest)
}

func addFileToTar(tarWriter *tar.Writer, filePath string, tarPath string, manifest *BackupManifest) error {
	fileInfo, errorValue := os.Stat(filePath)
	if errorValue != nil {
		return errorValue
	}
	if !fileInfo.Mode().IsRegular() {
		return nil
	}
	file, errorValue := os.Open(filePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	header, errorValue := tar.FileInfoHeader(fileInfo, "")
	if errorValue != nil {
		return errorValue
	}
	header.Name = tarPath
	if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	hasher := sha256.New()
	_, errorValue = io.Copy(tarWriter, io.TeeReader(file, hasher))
	if errorValue != nil {
		return errorValue
	}
	manifest.Checksums[tarPath] = hex.EncodeToString(hasher.Sum(nil))
	return nil
}

func addBytesToTar(tarWriter *tar.Writer, tarPath string, document []byte) error {
	header := &tar.Header{Name: tarPath, Mode: 0o600, Size: int64(len(document)), ModTime: time.Now()}
	if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	_, errorValue := tarWriter.Write(document)
	return errorValue
}

func extractBundle(bundlePath string, targetDirectoryPath string) (*BackupManifest, error) {
	bundleFile, errorValue := os.Open(bundlePath)
	if errorValue != nil {
		return nil, errorValue
	}
	defer bundleFile.Close()
	gzipReader, errorValue := gzip.NewReader(bundleFile)
	if errorValue != nil {
		return nil, errorValue
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var manifest *BackupManifest
	for {
		header, nextErrorValue := tarReader.Next()
		if nextErrorValue == io.EOF {
			break
		}
		if nextErrorValue != nil {
			return nil, nextErrorValue
		}
		if !isSafeTarPath(header.Name) {
			return nil, errors.New("unsafe backup path: " + header.Name)
		}
		targetPath := filepath.Join(targetDirectoryPath, header.Name)
		if header.Name == "manifest.json" {
			document, errorValue := io.ReadAll(tarReader)
			if errorValue != nil {
				return nil, errorValue
			}
			var parsedManifest BackupManifest
			if errorValue := json.Unmarshal(document, &parsedManifest); errorValue != nil {
				return nil, errorValue
			}
			manifest = &parsedManifest
			continue
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
			return nil, errorValue
		}
		file, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if errorValue != nil {
			return nil, errorValue
		}
		_, copyErrorValue := io.Copy(file, tarReader)
		closeErrorValue := file.Close()
		if copyErrorValue != nil {
			return nil, copyErrorValue
		}
		if closeErrorValue != nil {
			return nil, closeErrorValue
		}
	}
	if manifest == nil {
		return nil, errors.New("backup manifest is missing")
	}
	return manifest, nil
}

func encryptFile(inputPath string, outputPath string, passphrase string) error {
	plainDocument, errorValue := os.ReadFile(inputPath)
	if errorValue != nil {
		return errorValue
	}
	salt := randomBytes(16)
	nonce := randomBytes(12)
	key := deriveKey([]byte(passphrase), salt, 200000, 32)
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return errorValue
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		return errorValue
	}
	ciphertext := aead.Seal(nil, nonce, plainDocument, []byte("internkim-backup-v1"))
	outputDocument := append([]byte("IKBAK1\n"), salt...)
	outputDocument = append(outputDocument, nonce...)
	outputDocument = append(outputDocument, ciphertext...)
	return os.WriteFile(outputPath, outputDocument, 0o600)
}

func decryptFile(inputPath string, outputPath string, passphrase string) error {
	encryptedDocument, errorValue := os.ReadFile(inputPath)
	if errorValue != nil {
		return errorValue
	}
	if len(encryptedDocument) < len("IKBAK1\n")+16+12 || string(encryptedDocument[:7]) != "IKBAK1\n" {
		return errors.New("backup format is not supported")
	}
	offset := 7
	salt := encryptedDocument[offset : offset+16]
	offset += 16
	nonce := encryptedDocument[offset : offset+12]
	offset += 12
	ciphertext := encryptedDocument[offset:]
	key := deriveKey([]byte(passphrase), salt, 200000, 32)
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return errorValue
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		return errorValue
	}
	plainDocument, errorValue := aead.Open(nil, nonce, ciphertext, []byte("internkim-backup-v1"))
	if errorValue != nil {
		return errors.New("backup passphrase is incorrect or bundle is corrupted")
	}
	return os.WriteFile(outputPath, plainDocument, 0o600)
}

func deriveKey(password []byte, salt []byte, iterations int, keyLength int) []byte {
	var derivedKey []byte
	var block []byte
	blockIndex := 1
	for len(derivedKey) < keyLength {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(blockIndex >> 24), byte(blockIndex >> 16), byte(blockIndex >> 8), byte(blockIndex)})
		block = mac.Sum(nil)
		accumulator := append([]byte{}, block...)
		for iteration := 1; iteration < iterations; iteration++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(block)
			block = mac.Sum(nil)
			for index := range accumulator {
				accumulator[index] ^= block[index]
			}
		}
		derivedKey = append(derivedKey, accumulator...)
		blockIndex++
	}
	return derivedKey[:keyLength]
}

func copyDirectory(sourceRoot string, targetRoot string) error {
	return filepath.Walk(sourceRoot, func(sourcePath string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceRoot, sourcePath)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if information.IsDir() {
			return os.MkdirAll(targetPath, information.Mode())
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
			return errorValue
		}
		sourceFile, errorValue := os.Open(sourcePath)
		if errorValue != nil {
			return errorValue
		}
		defer sourceFile.Close()
		targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, information.Mode())
		if errorValue != nil {
			return errorValue
		}
		_, copyErrorValue := io.Copy(targetFile, sourceFile)
		closeErrorValue := targetFile.Close()
		if copyErrorValue != nil {
			return copyErrorValue
		}
		return closeErrorValue
	})
}

func copyRegularFile(sourcePath string, targetPath string) error {
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()
	sourceInfo, errorValue := sourceFile.Stat()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
		return errorValue
	}
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(targetFile, sourceFile)
	closeErrorValue := targetFile.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}

func isSafeTarPath(path string) bool {
	cleanPath := filepath.Clean(path)
	return cleanPath == path && !strings.HasPrefix(cleanPath, "..") && !filepath.IsAbs(cleanPath)
}

func tarName(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "/")
}
