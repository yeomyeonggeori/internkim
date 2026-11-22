package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	firstTakeOverRetryWait   = time.Minute
	longestTakeOverRetryWait = 30 * time.Minute
)

var errTakeOverWaitsToRetry = errors.New("the last take-over failed, and this box tries it again")

type takeOverAttempts struct {
	Retried       int       `json:"retried"`
	FailureSeenAt time.Time `json:"failureSeenAt"`
	SafetyBackup  string    `json:"safetyBackup,omitempty"`
	HeldNoCompany bool      `json:"heldNoCompany,omitempty"`
}

type takeOverRetry struct {
	StatePath string
	Now       func() time.Time
}

func takeOverRetries() takeOverRetry {
	return takeOverRetry{
		StatePath: filepath.Join(blueclaw.CompanyHostBoxStatePath, box.TakeOverRetryFileName),
		Now:       time.Now,
	}
}

func (retry takeOverRetry) allowARetryWhenItIsTime() error {
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
		return fmt.Errorf("%w at %s; `journalctl -u %s` says why it failed", errTakeOverWaitsToRetry, retryAt.Format(time.RFC3339), blueclaw.CompanyHostTakeOverServiceName)
	}
	return retry.write(takeOverAttempts{Retried: attempts.Retried + 1, SafetyBackup: attempts.SafetyBackup, HeldNoCompany: attempts.HeldNoCompany})
}

func (attempts takeOverAttempts) finding() companyhost.SafetyFinding {
	return companyhost.SafetyFinding{Archive: attempts.SafetyBackup, HeldNoCompany: attempts.HeldNoCompany}
}

func (retry takeOverRetry) keepTheFinding(finding companyhost.SafetyFinding) error {
	attempts, errorValue := retry.read()
	if errorValue != nil {
		return errorValue
	}
	attempts.SafetyBackup, attempts.HeldNoCompany = finding.Archive, finding.HeldNoCompany
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
