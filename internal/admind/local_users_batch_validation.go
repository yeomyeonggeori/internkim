package admind

import (
	"errors"
	"fmt"
	"strings"
)

func validateAdminUserBatchSize(users []adminUserMutation) error {
	if len(users) == 0 {
		return errors.New("users required")
	}
	if len(users) > localUsersBatchMaximumUsers {
		return fmt.Errorf("users exceeds maximum batch size of %d", localUsersBatchMaximumUsers)
	}
	return nil
}

func validateUniqueAdminUserEmails(emails []string) error {
	seenEmails := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		normalizedEmail := strings.ToLower(strings.TrimSpace(email))
		if _, found := seenEmails[normalizedEmail]; found {
			return fmt.Errorf("duplicate user email: %s", normalizedEmail)
		}
		seenEmails[normalizedEmail] = struct{}{}
	}
	return nil
}
