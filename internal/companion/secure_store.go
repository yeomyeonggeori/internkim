package companion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const developmentSecureStoreEnvironment = "INTERNKIM_COMPANION_DEV_FILE_STORE"

type SecureStore interface {
	Put(ctx context.Context, keyID string, secret string) error
	Get(ctx context.Context, keyID string) (string, error)
	Delete(ctx context.Context, keyID string) error
}

type MemorySecureStore struct {
	mutex   sync.Mutex
	secrets map[string]string
}

type KeychainSecureStore struct {
	Service string
}

type DevelopmentFileSecureStore struct {
	Directory string
}

type unavailableSecureStore struct {
	reason string
}

func NewMemorySecureStore() *MemorySecureStore {
	return &MemorySecureStore{secrets: map[string]string{}}
}

func NewDefaultSecureStore() SecureStore {
	if runtime.GOOS == "darwin" {
		return KeychainSecureStore{Service: "internkim"}
	}
	if os.Getenv(developmentSecureStoreEnvironment) == "1" {
		return DevelopmentFileSecureStore{Directory: defaultDevelopmentSecureStoreDirectory()}
	}
	return unavailableSecureStore{reason: "secure storage is unavailable; set INTERNKIM_COMPANION_DEV_FILE_STORE=1 only for development"}
}

func (store *MemorySecureStore) Put(ctx context.Context, keyID string, secret string) error {
	_ = ctx
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.secrets[keyID] = secret
	return nil
}

func (store *MemorySecureStore) Get(ctx context.Context, keyID string) (string, error) {
	_ = ctx
	store.mutex.Lock()
	defer store.mutex.Unlock()
	secret, ok := store.secrets[keyID]
	if !ok {
		return "", errors.New("secure secret not found")
	}
	return secret, nil
}

func (store *MemorySecureStore) Delete(ctx context.Context, keyID string) error {
	_ = ctx
	store.mutex.Lock()
	defer store.mutex.Unlock()
	delete(store.secrets, keyID)
	return nil
}

func (store KeychainSecureStore) Put(ctx context.Context, keyID string, secret string) error {
	return exec.CommandContext(ctx, "security", "add-generic-password", "-a", keyID, "-s", store.Service, "-w", secret, "-U").Run()
}

func (store KeychainSecureStore) Get(ctx context.Context, keyID string) (string, error) {
	output, errorValue := exec.CommandContext(ctx, "security", "find-generic-password", "-a", keyID, "-s", store.Service, "-w").Output()
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(output)), nil
}

func (store KeychainSecureStore) Delete(ctx context.Context, keyID string) error {
	return exec.CommandContext(ctx, "security", "delete-generic-password", "-a", keyID, "-s", store.Service).Run()
}

func (store DevelopmentFileSecureStore) Put(ctx context.Context, keyID string, secret string) error {
	_ = ctx
	if errorValue := os.MkdirAll(store.Directory, 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(filepath.Join(store.Directory, keyID+".secret"), []byte(secret), 0o600)
}

func (store DevelopmentFileSecureStore) Get(ctx context.Context, keyID string) (string, error) {
	_ = ctx
	document, errorValue := os.ReadFile(filepath.Join(store.Directory, keyID+".secret"))
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(document)), nil
}

func (store DevelopmentFileSecureStore) Delete(ctx context.Context, keyID string) error {
	_ = ctx
	errorValue := os.Remove(filepath.Join(store.Directory, keyID+".secret"))
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	return errorValue
}

func (store unavailableSecureStore) Put(ctx context.Context, keyID string, secret string) error {
	_ = ctx
	_ = keyID
	_ = secret
	return errors.New(store.reason)
}

func (store unavailableSecureStore) Get(ctx context.Context, keyID string) (string, error) {
	_ = ctx
	_ = keyID
	return "", errors.New(store.reason)
}

func (store unavailableSecureStore) Delete(ctx context.Context, keyID string) error {
	_ = ctx
	_ = keyID
	return errors.New(store.reason)
}

func defaultDevelopmentSecureStoreDirectory() string {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil || homeDirectory == "" {
		return ".internkim-companion-secure"
	}
	return filepath.Join(homeDirectory, ".internkim-companion", "secure")
}
