package admind

func mattermostDisplayName(userRecord mattermostUserRecord) string {
	return firstNonEmpty(userRecord.DisplayName, userRecord.Nickname, userRecord.FirstName, userRecord.Username)
}
