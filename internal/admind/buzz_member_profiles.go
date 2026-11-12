package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/personname"
)

func (service *Service) nameEveryoneTheDirectoryNames(ctx context.Context) error {
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return errorValue
	}
	profiles, errorValue := latestProfilesByPubkey(ctx, relay)
	if errorValue != nil {
		return fmt.Errorf("reading the names the relay holds: %w", errorValue)
	}
	recordedNames := service.recordedNamesByEmail(ctx)
	language := service.workspaceLanguage(ctx)
	var refusals []error
	for _, email := range service.everyAddressThisDeviceKnows(ctx) {
		refusals = append(refusals, service.nameMemberFromTheDirectory(ctx, profiles, email, recordedNames[email], language))
	}
	return errors.Join(refusals...)
}

// The person an admin just added is in the company now, and the company's rooms
// are where that shows. Their own sign-in and the next pass would both get
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
	if errorValue := service.nameMemberFromTheDirectory(ctx, profiles, email, recordedName, service.workspaceLanguage(ctx)); errorValue != nil {
		log.Printf("buzz profiles: %v", errorValue)
	}
}

func (service *Service) nameMemberFromTheDirectory(
	ctx context.Context,
	profiles map[string]buzzProfileContent,
	email string,
	recordedName string,
	language string,
) error {
	secretHex, displayName := service.buzzProfileOwed(ctx, profiles, email, recordedName, language)
	if displayName == "" {
		return nil
	}
	if errorValue := service.publishBuzzProfile(ctx, secretHex, displayName, ""); errorValue != nil {
		return fmt.Errorf("%s stays a key: %w", email, errorValue)
	}
	log.Printf("buzz profiles: %s is %s", email, displayName)
	return nil
}

func (service *Service) buzzProfileOwed(
	ctx context.Context,
	profiles map[string]buzzProfileContent,
	email string,
	recordedName string,
	language string,
) (string, string) {
	if personname.Render(recordedName, language) == "" {
		return "", ""
	}
	secretHex := service.buzzSecretForEmail(ctx, email)
	if secretHex == "" {
		return "", ""
	}
	pubkey, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		return "", ""
	}
	return secretHex, buzzProfileNameToWrite(profiles[pubkey], recordedName, language)
}

func (service *Service) publishBuzzProfile(ctx context.Context, secretHex string, displayName string, pictureURL string) error {
	publisher, errorValue := service.connectToTheRelayOnceItAnswers(ctx, secretHex)
	if errorValue != nil {
		return errorValue
	}
	defer publisher.Close()
	return publisher.SetProfile(ctx, secretHex, displayName, pictureURL)
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
