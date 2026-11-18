package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	firstTakeOverRetryWait   = time.Minute
	longestTakeOverRetryWait = 30 * time.Minute
	takeOverRetryFileName    = "take-over-retry.json"
)

var errTakeOverWaitsToRetry = errors.New("the last take-over failed, and this box tries it again")

type takeOverAttempts struct {
	Retried       int       `json:"retried"`
	FailureSeenAt time.Time `json:"failureSeenAt"`
	SafetyBackup  string    `json:"safetyBackup,omitempty"`
}

type takeOverRetry struct {
	StatePath   string
	Now         func() time.Time
	ResetFailed func() error
}

func takeOverRetries() takeOverRetry {
	return takeOverRetry{
		StatePath:   filepath.Join(blueclaw.CompanyHostBoxStatePath, takeOverRetryFileName),
		Now:         time.Now,
		ResetFailed: resetTheFailedTakeOver,
	}
}

func resetTheFailedTakeOver() error {
	output, errorValue := exec.Command("systemctl", "reset-failed", takeOverUnitName+".service").CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("systemctl reset-failed %s: %v: %s", takeOverUnitName, errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}

func (retry takeOverRetry) clearTheFailedUnitWhenItIsTime() error {
	attempts, errorValue := retry.read()
	if errorValue != nil {
		return errorValue
	}
	if attempts.FailureSeenAt.IsZero() {
		attempts.FailureSeenAt = retry.Now()
		if errorValue := retry.write(attempts); errorValue != nil {
			return errorValue
		}
	}
	retryAt := attempts.FailureSeenAt.Add(takeOverRetryWait(attempts.Retried))
	if retry.Now().Before(retryAt) {
		return fmt.Errorf("%w at %s; `journalctl -u %s` says why it failed", errTakeOverWaitsToRetry, retryAt.Format(time.RFC3339), takeOverUnitName)
	}
	if errorValue := retry.ResetFailed(); errorValue != nil {
		return fmt.Errorf("clearing the failed take-over to try it again: %w", errorValue)
	}
	return retry.write(takeOverAttempts{Retried: attempts.Retried + 1, SafetyBackup: attempts.SafetyBackup})
}

func (retry takeOverRetry) keepSafetyBackup(archivePath string) error {
	attempts, errorValue := retry.read()
	if errorValue != nil {
		return errorValue
	}
	attempts.SafetyBackup = archivePath
	return retry.write(attempts)
}

func (retry takeOverRetry) finishTheTakeOver() error {
	if errorValue := retry.forget(); errorValue != nil {
		return fmt.Errorf("the take-over succeeded, and its record at %s could not be removed: %w", retry.StatePath, errorValue)
	}
	return nil
}

func (retry takeOverRetry) forget() error {
	errorValue := os.Remove(retry.StatePath)
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	return errorValue
}

func takeOverRetryWait(retried int) time.Duration {
	wait := firstTakeOverRetryWait
	for range retried {
		wait *= 2
		if wait >= longestTakeOverRetryWait {
			return longestTakeOverRetryWait
		}
	}
	return wait
}

func (retry takeOverRetry) read() (takeOverAttempts, error) {
	document, errorValue := os.ReadFile(retry.StatePath)
	if errors.Is(errorValue, os.ErrNotExist) {
		return takeOverAttempts{}, nil
	}
	if errorValue != nil {
		return takeOverAttempts{}, errorValue
	}
	var attempts takeOverAttempts
	if errorValue := json.Unmarshal(document, &attempts); errorValue != nil {
		return takeOverAttempts{}, fmt.Errorf("the take-over attempts at %s are not ones this box wrote: %w", retry.StatePath, errorValue)
	}
	return attempts, nil
}

func (retry takeOverRetry) write(attempts takeOverAttempts) error {
	document, errorValue := json.Marshal(attempts)
	if errorValue != nil {
		return errorValue
	}
	return durablefile.Write(retry.StatePath, document, 0o600)
}
