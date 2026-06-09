package tenantruntime

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const backupHeader = "internkim-tenant-backup-v1\n"

func (service Service) CreateEncryptedBackup(tenantID string, bundlePath string, passphrase string) error {
	if strings.TrimSpace(passphrase) == "" {
		return errors.New("backup passphrase is required")
	}
	manifest, errorValue := service.ReadManifest(tenantID)
	if errorValue != nil {
		return errorValue
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		return errorValue
	}
	archiveDocument, errorValue := archiveTenant(paths.TenantRootPath)
	if errorValue != nil {
		return errorValue
	}
	encryptedDocument, errorValue := encryptBackup(archiveDocument, passphrase)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(bundlePath), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(bundlePath, encryptedDocument, 0o600)
}

func (service Service) RestoreEncryptedBackup(bundlePath string, passphrase string) (TenantStatus, error) {
	if strings.TrimSpace(passphrase) == "" {
		return TenantStatus{}, errors.New("backup passphrase is required")
	}
	encryptedDocument, errorValue := os.ReadFile(bundlePath)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	archiveDocument, errorValue := decryptBackup(encryptedDocument, passphrase)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	tenantID, errorValue := restoreTenantArchive(service.BasePath, archiveDocument)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, tenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := createRuntimeDirectories(paths); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	return service.Status(tenantID)
}

func archiveTenant(tenantRootPath string) ([]byte, error) {
	var archiveBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&archiveBuffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if errorValue := writeArchiveEntry(tarWriter, filepath.Join(tenantRootPath, "manifest.json"), "manifest.json"); errorValue != nil {
		return nil, errorValue
	}
	walkError := filepath.WalkDir(tenantRootPath, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(tenantRootPath, path)
		if errorValue != nil {
			return errorValue
		}
		if relativePath == "." || relativePath == "manifest.json" || relativePath == "backups" || strings.HasPrefix(relativePath, "backups"+string(filepath.Separator)) {
			return nil
		}
		return writeArchiveEntry(tarWriter, path, filepath.ToSlash(relativePath))
	})
	if walkError != nil {
		return nil, walkError
	}
	if errorValue := tarWriter.Close(); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := gzipWriter.Close(); errorValue != nil {
		return nil, errorValue
	}
	return archiveBuffer.Bytes(), nil
}

func writeArchiveEntry(tarWriter *tar.Writer, path string, archivePath string) error {
	info, errorValue := os.Stat(path)
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := tar.FileInfoHeader(info, "")
	if errorValue != nil {
		return errorValue
	}
	header.Name = archivePath
	if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	if info.IsDir() {
		return nil
	}
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = io.Copy(tarWriter, file)
	return errorValue
}

func restoreTenantArchive(basePath string, archiveDocument []byte) (string, error) {
	gzipReader, errorValue := gzip.NewReader(bytes.NewReader(archiveDocument))
	if errorValue != nil {
		return "", errorValue
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	tenantID := ""
	for {
		header, errorValue := tarReader.Next()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil {
			return "", errorValue
		}
		if !isSafeArchivePath(header.Name) {
			return "", errors.New("backup contains unsafe path")
		}
		targetPath := filepath.Join(filepath.Clean(basePath), filepath.FromSlash(header.Name))
		if header.Name == "manifest.json" {
			manifestDocument, errorValue := readTarFile(tarReader)
			if errorValue != nil {
				return "", errorValue
			}
			manifest, errorValue := manifestFromDocument(manifestDocument)
			if errorValue != nil {
				return "", errorValue
			}
			tenantID = manifest.TenantID
			targetPath = filepath.Join(filepath.Clean(basePath), tenantID, "manifest.json")
			if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o750); errorValue != nil {
				return "", errorValue
			}
			if errorValue := os.WriteFile(targetPath, manifestDocument, 0o600); errorValue != nil {
				return "", errorValue
			}
			continue
		}
		if tenantID == "" {
			return "", errors.New("backup manifest must appear before tenant files")
		}
		targetPath = filepath.Join(filepath.Clean(basePath), tenantID, filepath.FromSlash(header.Name))
		if errorValue := restoreTarEntry(targetPath, header, tarReader); errorValue != nil {
			return "", errorValue
		}
	}
	if tenantID == "" {
		return "", errors.New("backup manifest is missing")
	}
	return tenantID, nil
}

func restoreTarEntry(targetPath string, header *tar.Header, reader io.Reader) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(targetPath, os.FileMode(header.Mode).Perm())
	case tar.TypeReg:
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o750); errorValue != nil {
			return errorValue
		}
		file, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode).Perm())
		if errorValue != nil {
			return errorValue
		}
		defer file.Close()
		_, errorValue = io.Copy(file, reader)
		return errorValue
	default:
		return nil
	}
}

func readTarFile(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

func manifestFromDocument(document []byte) (Manifest, error) {
	var manifest Manifest
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return Manifest{}, errorValue
	}
	if errorValue := manifest.Validate(); errorValue != nil {
		return Manifest{}, errorValue
	}
	return manifest, nil
}

func isSafeArchivePath(path string) bool {
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	return cleanPath != "." && !filepath.IsAbs(cleanPath) && !strings.HasPrefix(cleanPath, "..")
}

func encryptBackup(document []byte, passphrase string) ([]byte, error) {
	aesGCM, errorValue := backupCipher(passphrase)
	if errorValue != nil {
		return nil, errorValue
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, errorValue := rand.Read(nonce); errorValue != nil {
		return nil, errorValue
	}
	ciphertext := aesGCM.Seal(nil, nonce, document, []byte(backupHeader))
	result := append([]byte(backupHeader), nonce...)
	return append(result, ciphertext...), nil
}

func decryptBackup(document []byte, passphrase string) ([]byte, error) {
	if !bytes.HasPrefix(document, []byte(backupHeader)) {
		return nil, errors.New("backup format is not supported")
	}
	aesGCM, errorValue := backupCipher(passphrase)
	if errorValue != nil {
		return nil, errorValue
	}
	payload := bytes.TrimPrefix(document, []byte(backupHeader))
	if len(payload) < aesGCM.NonceSize() {
		return nil, errors.New("backup payload is incomplete")
	}
	nonce := payload[:aesGCM.NonceSize()]
	ciphertext := payload[aesGCM.NonceSize():]
	return aesGCM.Open(nil, nonce, ciphertext, []byte(backupHeader))
}

func backupCipher(passphrase string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(passphrase))
	block, errorValue := aes.NewCipher(key[:])
	if errorValue != nil {
		return nil, errorValue
	}
	return cipher.NewGCM(block)
}
