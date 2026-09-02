package admind

import (
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostadmin"
)

func mattermostDisplayName(userRecord mattermostadmin.UserRecord) string {
	return firstNonEmpty(userRecord.DisplayName, userRecord.Nickname, userRecord.FirstName, userRecord.Username)
}
