package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

type personPicture struct {
	Email      string `json:"email"`
	Name       string `json:"name,omitempty"`
	PictureURL string `json:"pictureURL,omitempty"`
}

type personPicturesResponse struct {
	Pictures []personPicture `json:"pictures"`
}

// Every screen that draws a person draws them through one directory: the
// messenger's own profiles, keyed by the address the company knows them by.
func (service *Service) handlePersonPictures(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	if service.webActorEmail(request) == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	pictures, errorValue := service.personPicturesFromRelay(request.Context())
	if errorValue != nil {
		log.Printf("person pictures failed: %v", errorValue)
		http.Error(responseWriter, "person_pictures_failed", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, personPicturesResponse{Pictures: pictures})
}

func (service *Service) personPicturesFromRelay(ctx context.Context) ([]personPicture, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return nil, errBuzzKeySeedMissing
	}
	emails, errorValue := service.companyPeopleEmails(ctx)
	if errorValue != nil {
		return nil, errorValue
	}

	database, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return nil, errorValue
	}
	profiles, errorValue := latestProfilesByPubkey(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}

	pictures := []personPicture{}
	for _, email := range emails {
		vaultSubject, errorValue := service.buzzVaultSubject(ctx, email)
		if errorValue != nil {
			return nil, errorValue
		}
		version := service.buzzIdentityVersion(vaultSubject)
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, version)))
		if errorValue != nil {
			return nil, errorValue
		}
		profile := profiles[pubkey]
		pictures = append(pictures, personPicture{
			Email:      email,
			Name:       firstNonEmpty(profile.Display, profile.Name),
			PictureURL: service.rewriteBuzzMedia(profile.Picture),
		})
	}
	return pictures, nil
}

type buzzProfileContent struct {
	Name    string `json:"name"`
	Display string `json:"display_name"`
	Picture string `json:"picture"`
}

func latestProfilesByPubkey(ctx context.Context, database *sql.DB) (map[string]buzzProfileContent, error) {
	rows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT ON (pubkey) encode(pubkey,'hex'), content FROM events WHERE kind = 0 ORDER BY pubkey, created_at DESC")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	profiles := map[string]buzzProfileContent{}
	for rows.Next() {
		var pubkey, content string
		if errorValue := rows.Scan(&pubkey, &content); errorValue != nil {
			return nil, errorValue
		}
		var profile buzzProfileContent
		if errorValue := json.Unmarshal([]byte(content), &profile); errorValue != nil {
			continue
		}
		profiles[pubkey] = profile
	}
	return profiles, rows.Err()
}

// The directory says who works here, and somebody invited on the web is in it
// before this device has heard of them any other way. The policy is read beside
// it because it carries whoever was here before the directory did.
func (service *Service) companyPeopleEmails(ctx context.Context) ([]string, error) {
	emails := []string{}
	seen := map[string]bool{}
	add := func(email string) {
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized == "" || seen[normalized] {
			return
		}
		seen[normalized] = true
		emails = append(emails, normalized)
	}

	client := service.centralPlane()
	if client != nil {
		members, errorValue := client.Members(ctx)
		if errorValue != nil {
			return nil, errorValue
		}
		for _, member := range members {
			add(member.Email)
		}
	}

	policyEmails, errorValue := service.policyPeopleEmails(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	for _, email := range policyEmails {
		add(email)
	}
	return emails, nil
}

func (service *Service) policyPeopleEmails(ctx context.Context) ([]string, error) {
	var policyDocument memoryPolicyDocument
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	emails := []string{}
	for _, person := range policyDocument.People {
		emails = append(emails, person.Emails...)
	}
	return emails, nil
}
