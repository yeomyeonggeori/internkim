package companyhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	AdminAccountName       = "admin"
	adminAccountShell      = "/bin/bash"
	journalReadersGroup    = "systemd-journal"
	adminAccountRecordFile = "admin-account"
)

type AdminAccount struct {
	StateDirectoryPath string
	Run                func(ctx context.Context, input string, name string, arguments ...string) error
	LookupUserID       func(name string) (string, error)
}

func (account AdminAccount) Ensure(ctx context.Context) (bool, error) {
	userID, errorValue := account.lookupUserID()
	if isUnknownUser(errorValue) {
		return true, account.create(ctx)
	}
	if errorValue != nil {
		return false, fmt.Errorf("looking up the %s account: %w", AdminAccountName, errorValue)
	}
	return userID == account.recordedUserID(), nil
}

func (account AdminAccount) Adopt() error {
	if account.recordedUserID() != "" {
		return nil
	}
	userID, errorValue := account.lookupUserID()
	if isUnknownUser(errorValue) {
		return nil
	}
	if errorValue != nil {
		return fmt.Errorf("looking up the %s account: %w", AdminAccountName, errorValue)
	}
	return account.record(userID)
}

func (account AdminAccount) SetPassword(ctx context.Context, password string) error {
	if errorValue := account.run(ctx, AdminAccountName+":"+password+"\n", "chpasswd"); errorValue != nil {
		return fmt.Errorf("chpasswd refused the password for %s: %w", AdminAccountName, errorValue)
	}
	return nil
}

func (account AdminAccount) Lock(ctx context.Context) error {
	if errorValue := account.run(ctx, "", "passwd", "--lock", AdminAccountName); errorValue != nil {
		return fmt.Errorf("passwd could not lock %s: %w", AdminAccountName, errorValue)
	}
	return nil
}

func (account AdminAccount) create(ctx context.Context) error {
	if errorValue := account.run(ctx, "", "useradd", adminAccountCreation()...); errorValue != nil {
		return fmt.Errorf("creating the %s account: %w", AdminAccountName, errorValue)
	}
	userID, errorValue := account.lookupUserID()
	if errorValue != nil {
		return fmt.Errorf("looking up the %s account just created: %w", AdminAccountName, errorValue)
	}
	if errorValue := account.record(userID); errorValue != nil {
		return errorValue
	}
	return account.Lock(ctx)
}

func adminAccountCreation() []string {
	return []string{
		"--create-home",
		"--shell", adminAccountShell,
		"--groups", journalReadersGroup + "," + blueclaw.CompanyHostAdministratorGroup,
		AdminAccountName,
	}
}

func (account AdminAccount) recordedUserID() string {
	document, errorValue := os.ReadFile(account.recordPath())
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func (account AdminAccount) record(userID string) error {
	if errorValue := os.MkdirAll(account.StateDirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := durablefile.Write(account.recordPath(), []byte(userID+"\n"), 0o600); errorValue != nil {
		return fmt.Errorf("remembering that InternKim made the %s account: %w", AdminAccountName, errorValue)
	}
	return nil
}

func (account AdminAccount) recordPath() string {
	return filepath.Join(account.StateDirectoryPath, adminAccountRecordFile)
}

func (account AdminAccount) lookupUserID() (string, error) {
	if account.LookupUserID != nil {
		return account.LookupUserID(AdminAccountName)
	}
	found, errorValue := user.Lookup(AdminAccountName)
	if errorValue != nil {
		return "", errorValue
	}
	return found.Uid, nil
}

func isUnknownUser(errorValue error) bool {
	var unknown user.UnknownUserError
	return errors.As(errorValue, &unknown)
}

func (account AdminAccount) run(ctx context.Context, input string, name string, arguments ...string) error {
	if account.Run != nil {
		return account.Run(ctx, input, name, arguments...)
	}
	command := exec.CommandContext(ctx, name, arguments...)
	command.Stdin = strings.NewReader(input)
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		return fmt.Errorf("%w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}
