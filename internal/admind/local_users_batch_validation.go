package admind

import (
	"fmt"
	"strings"
)

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
