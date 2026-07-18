package admind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	calendarTokenResetQuarantineSuffix = ".reset-pending"
	calendarTokenAccountPendingSuffix  = ".account-pending"
)

func (service *Service) recoverGoogleOAuthTokenFilesLocked(ctx context.Context) error {
	if errorValue := service.recoverCalendarTokenResetQuarantine(ctx); errorValue != nil {
		return errorValue
	}
	return service.recoverPendingGoogleOAuthAccountTokens(ctx)
}

func (service *Service) recoverPendingGoogleOAuthAccountTokens(ctx context.Context) error {
	pattern := filepath.Join(service.calendarSecretsDirectory(), "*"+calendarTokenFileExtension+calendarTokenAccountPendingSuffix)
	paths, errorValue := filepath.Glob(pattern)
	if errorValue != nil {
		return fmt.Errorf("find pending google calendar token files: %w", errorValue)
	}
	if len(paths) == 0 {
		return nil
	}
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return fmt.Errorf("load google calendar token encryption key: %w", errorValue)
	}
	accounts, errorValue := service.listRemoteCalendarAccountsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return fmt.Errorf("read google calendar accounts before pending token recovery: %w", errorValue)
	}
	activeTokenPaths := map[string]bool{}
	for _, account := range accounts {
		activeTokenPaths[filepath.Clean(strings.TrimSpace(account.TokenFilePath))] = true
	}
	for _, path := range paths {
		originalPath := strings.TrimSuffix(path, calendarTokenAccountPendingSuffix)
		if activeTokenPaths[filepath.Clean(originalPath)] {
			if errorValue := validateCalendarTokenFile(originalPath, key); errorValue == nil {
				if errorValue := deleteCalendarTokenFile(path); errorValue != nil {
					return fmt.Errorf("delete committed pending google calendar token file %s: %w", filepath.Base(path), errorValue)
				}
				continue
			} else if !errors.Is(errorValue, os.ErrNotExist) {
				if pendingError := validateCalendarTokenFile(path, key); pendingError != nil {
					return errors.Join(
						fmt.Errorf("validate active google calendar token file %s: %w", filepath.Base(originalPath), errorValue),
						fmt.Errorf("validate pending google calendar token file %s: %w", filepath.Base(path), pendingError),
					)
				}
				backupPath, backupError := service.quarantineCalendarTokenForReset(originalPath)
				if backupError != nil {
					return fmt.Errorf("quarantine invalid google calendar token file %s: %w", filepath.Base(originalPath), backupError)
				}
				if errorValue := service.promotePendingGoogleOAuthToken(path, originalPath); errorValue != nil {
					return errors.Join(
						fmt.Errorf("activate pending google calendar token file %s: %w", filepath.Base(path), errorValue),
						restoreCalendarTokenFromResetQuarantine(originalPath, backupPath),
					)
				}
				if errorValue := validateCalendarTokenFile(originalPath, key); errorValue != nil {
					return fmt.Errorf("validate recovered google calendar token file %s: %w", filepath.Base(originalPath), errorValue)
				}
				if errorValue := deleteCalendarTokenFile(path); errorValue != nil {
					return fmt.Errorf("delete recovered pending google calendar token file %s: %w", filepath.Base(path), errorValue)
				}
				if errorValue := service.removeCalendarTokenResetQuarantine(backupPath); errorValue != nil {
					return fmt.Errorf("delete invalid google calendar token backup %s: %w", filepath.Base(backupPath), errorValue)
				}
				continue
			}
			if errorValue := validateCalendarTokenFile(path, key); errorValue != nil {
				return fmt.Errorf("validate pending google calendar token file %s: %w", filepath.Base(path), errorValue)
			}
			if errorValue := service.promotePendingGoogleOAuthToken(path, originalPath); errorValue != nil {
				return fmt.Errorf("activate pending google calendar token file %s: %w", filepath.Base(path), errorValue)
			}
			if errorValue := validateCalendarTokenFile(originalPath, key); errorValue != nil {
				return fmt.Errorf("validate recovered google calendar token file %s: %w", filepath.Base(originalPath), errorValue)
			}
			if errorValue := deleteCalendarTokenFile(path); errorValue != nil {
				return fmt.Errorf("delete recovered pending google calendar token file %s: %w", filepath.Base(path), errorValue)
			}
			continue
		}
		if errorValue := deleteCalendarTokenFile(originalPath); errorValue != nil {
			return fmt.Errorf("delete orphaned active google calendar token file %s: %w", filepath.Base(originalPath), errorValue)
		}
		if errorValue := deleteCalendarTokenFile(path); errorValue != nil {
			return fmt.Errorf("delete orphaned pending google calendar token file %s: %w", filepath.Base(path), errorValue)
		}
	}
	return nil
}

func (service *Service) promotePendingGoogleOAuthToken(pendingPath string, tokenPath string) error {
	if service.promoteCalendarTokenFile != nil {
		return service.promoteCalendarTokenFile(pendingPath, tokenPath)
	}
	document, errorValue := os.ReadFile(pendingPath)
	if errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(tokenPath, document, 0o600)
}

func (service *Service) recoverCalendarTokenResetQuarantine(ctx context.Context) error {
	pattern := filepath.Join(service.calendarSecretsDirectory(), "*"+calendarTokenFileExtension+calendarTokenResetQuarantineSuffix)
	paths, errorValue := filepath.Glob(pattern)
	if errorValue != nil {
		return fmt.Errorf("find quarantined google calendar token files: %w", errorValue)
	}
	if len(paths) == 0 {
		return nil
	}
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return fmt.Errorf("load google calendar token encryption key: %w", errorValue)
	}
	accounts, errorValue := service.listRemoteCalendarAccountsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return fmt.Errorf("read google calendar accounts before token recovery: %w", errorValue)
	}
	activeTokenPaths := map[string]bool{}
	for _, account := range accounts {
		activeTokenPaths[filepath.Clean(strings.TrimSpace(account.TokenFilePath))] = true
	}
	for _, path := range paths {
		originalPath := strings.TrimSuffix(path, calendarTokenResetQuarantineSuffix)
		if activeTokenPaths[filepath.Clean(originalPath)] {
			if errorValue := validateCalendarTokenFile(originalPath, key); errorValue == nil {
				if errorValue := service.removeCalendarTokenResetQuarantine(path); errorValue != nil {
					return fmt.Errorf("delete superseded google calendar token file %s: %w", filepath.Base(path), errorValue)
				}
				continue
			} else if !errors.Is(errorValue, os.ErrNotExist) {
				if quarantineError := validateCalendarTokenFile(path, key); quarantineError != nil {
					return errors.Join(
						fmt.Errorf("validate active google calendar token file %s: %w", filepath.Base(originalPath), errorValue),
						fmt.Errorf("validate quarantined google calendar token file %s: %w", filepath.Base(path), quarantineError),
					)
				}
				if deleteError := deleteCalendarTokenFile(originalPath); deleteError != nil {
					return errors.Join(
						fmt.Errorf("validate active google calendar token file %s: %w", filepath.Base(originalPath), errorValue),
						fmt.Errorf("delete invalid active google calendar token file %s: %w", filepath.Base(originalPath), deleteError),
					)
				}
			} else if errorValue := validateCalendarTokenFile(path, key); errorValue != nil {
				return fmt.Errorf("validate quarantined google calendar token file %s: %w", filepath.Base(path), errorValue)
			}
			if errorValue := renameCalendarTokenFile(path, originalPath); errorValue != nil {
				return fmt.Errorf("restore quarantined google calendar token file %s: %w", filepath.Base(path), errorValue)
			}
			if errorValue := validateCalendarTokenFile(originalPath, key); errorValue != nil {
				return fmt.Errorf("validate restored google calendar token file %s: %w", filepath.Base(originalPath), errorValue)
			}
			continue
		}
		if errorValue := service.removeCalendarTokenResetQuarantine(path); errorValue != nil {
			return fmt.Errorf("delete quarantined google calendar token file %s: %w", filepath.Base(path), errorValue)
		}
	}
	return nil
}

func (service *Service) removeCalendarTokenResetQuarantine(path string) error {
	if service.removeTokenQuarantineFile != nil {
		return service.removeTokenQuarantineFile(path)
	}
	return deleteCalendarTokenFile(path)
}

func (service *Service) quarantineCalendarTokenForReset(path string) (string, error) {
	if !service.canDeleteCalendarTokenFile(path) {
		return "", nil
	}
	fileInformation, errorValue := os.Lstat(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return "", nil
	}
	if errorValue != nil {
		return "", errorValue
	}
	if !fileInformation.Mode().IsRegular() {
		return "", fmt.Errorf("google calendar token path is not a regular file")
	}
	quarantinePath := path + calendarTokenResetQuarantineSuffix
	if errorValue := renameCalendarTokenFile(path, quarantinePath); errorValue != nil {
		return "", errorValue
	}
	return quarantinePath, nil
}

func validateCalendarTokenFile(path string, key []byte) error {
	fileInformation, errorValue := os.Lstat(path)
	if errorValue != nil {
		return errorValue
	}
	if !fileInformation.Mode().IsRegular() {
		return fmt.Errorf("calendar token path is not a regular file")
	}
	payload, errorValue := readCalendarTokenFile(path, key)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(payload.AccessToken) == "" && strings.TrimSpace(payload.RefreshToken) == "" {
		return fmt.Errorf("calendar token payload has no access or refresh token")
	}
	return nil
}

func restoreCalendarTokenFromResetQuarantine(originalPath string, quarantinePath string) error {
	if quarantinePath == "" {
		return nil
	}
	return renameCalendarTokenFile(quarantinePath, originalPath)
}
