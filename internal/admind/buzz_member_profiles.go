package admind

import (
	"context"
	"database/sql"
	"log"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/personname"
)

// A messenger that cannot name somebody draws the first bytes of their key
// instead, which is what everyone invited here has looked like: the import
// wrote a profile for each person it carried over, and nothing wrote one for
// anybody who arrived afterwards. The company directory is what
// names a person, so a member the relay cannot name gets a profile in their own
// name, signed by their own key.
func (service *Service) nameMembersTheRelayCannotName(ctx context.Context, relay *sql.DB) {
	profiles, errorValue := latestProfilesByPubkey(ctx, relay)
	if errorValue != nil {
		log.Printf("buzz profiles: reading the names the relay holds failed: %v", errorValue)
		return
	}
	recordedNames := service.recordedNamesByEmail(ctx)
	language := service.workspaceLanguage(ctx)
	named, unnamed := 0, 0
	for _, email := range service.everyAddressThisDeviceKnows(ctx) {
		if recordedNames[email] == "" {
			unnamed++
		}
		if service.nameMemberFromTheDirectory(ctx, profiles, email, recordedNames[email], language) {
			named++
		}
	}
	log.Printf("buzz profiles: named %d member(s), and the directory names %d of them nothing", named, unnamed)
}

// The person an admin just added is in the company now, and the company's rooms
// are where that shows. Their own sign-in and the daily pass would both get
// there eventually; neither is soon enough to keep them from appearing as a key.
func (service *Service) seatAndNameOneMemberInBuzz(ctx context.Context, email string, recordedName string) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	service.ensureUserChannelMembership(ctx, email)
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		log.Printf("buzz profiles: %s stays a key: %v", email, errorValue)
		return
	}
	profiles, errorValue := latestProfilesByPubkey(ctx, relay)
	if errorValue != nil {
		log.Printf("buzz profiles: reading the names the relay holds failed: %v", errorValue)
		return
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if strings.TrimSpace(recordedName) == "" {
		recordedName = service.recordedNamesByEmail(ctx)[email]
	}
	service.nameMemberFromTheDirectory(ctx, profiles, email, recordedName, service.workspaceLanguage(ctx))
}

func (service *Service) nameMemberFromTheDirectory(
	ctx context.Context,
	profiles map[string]buzzProfileContent,
	email string,
	recordedName string,
	language string,
) bool {
	if personname.Render(recordedName, language) == "" {
		return false
	}
	secretHex := service.buzzSecretForEmail(ctx, email)
	if secretHex == "" {
		return false
	}
	pubkey, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		return false
	}
	displayName := buzzProfileNameToWrite(profiles[pubkey], recordedName, language)
	if displayName == "" {
		return false
	}
	if errorValue := service.publishBuzzProfile(ctx, secretHex, displayName); errorValue != nil {
		log.Printf("buzz profiles: %s stays a key: %v", email, errorValue)
		return false
	}
	log.Printf("buzz profiles: %s is %s", email, displayName)
	return true
}

func (service *Service) publishBuzzProfile(ctx context.Context, secretHex string, displayName string) error {
	publisher, errorValue := service.connectToTheRelayOnceItAnswers(ctx, secretHex)
	if errorValue != nil {
		return errorValue
	}
	defer publisher.Close()
	return publisher.SetProfile(ctx, secretHex, displayName)
}

func (service *Service) recordedNamesByEmail(ctx context.Context) map[string]string {
	names := map[string]string{}
	for _, record := range service.directoryRecords(ctx) {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		name := strings.TrimSpace(record.Name)
		if email == "" || name == "" || names[email] != "" {
			continue
		}
		names[email] = name
	}
	return names
}

// A name somebody chose for themselves is theirs, so the directory writes only
// where the relay holds no name at all.
func buzzProfileNameToWrite(profile buzzProfileContent, recordedName string, language string) string {
	if firstNonEmpty(profile.Display, profile.Name) != "" {
		return ""
	}
	return personname.Render(recordedName, language)
}
