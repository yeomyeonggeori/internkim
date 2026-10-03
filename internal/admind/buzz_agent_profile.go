package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
	"github.com/yeomyeonggeori/internkim/internal/buzzimport/media"
)

var errAgentPictureIsNotAnImage = errors.New("the agent's default picture is not an image")

type agentProfileMessenger interface {
	heldProfile(ctx context.Context) (buzzProfileContent, error)
	uploadPicture(ctx context.Context, content []byte, mimeType string) (string, error)
	publishProfile(ctx context.Context, displayName string, pictureURL string) error
}

func giveTheAgentADefaultPicture(ctx context.Context, messenger agentProfileMessenger, picture []byte) (bool, error) {
	held, errorValue := messenger.heldProfile(ctx)
	if errorValue != nil {
		return false, fmt.Errorf("reading the agent's profile: %w", errorValue)
	}
	if held.Picture != "" {
		return false, nil
	}
	content, mimeType, isImage := media.StripMetadata(picture, http.DetectContentType(picture))
	if !isImage {
		return false, errAgentPictureIsNotAnImage
	}
	pictureURL, errorValue := messenger.uploadPicture(ctx, content, mimeType)
	if errorValue != nil {
		return false, fmt.Errorf("uploading the agent's picture: %w", errorValue)
	}
	if errorValue := messenger.publishProfile(ctx, firstNonEmpty(held.Display, held.Name, agentName), pictureURL); errorValue != nil {
		return false, fmt.Errorf("publishing the agent's profile: %w", errorValue)
	}
	return true, nil
}

func (service *Service) publishTheAgentProfile(ctx context.Context) {
	picturePath := strings.TrimSpace(service.Configuration.AgentProfilePicturePath)
	if picturePath == "" {
		return
	}
	messenger, errorValue := service.relayAgentProfile()
	if errorValue != nil {
		log.Printf("buzz agent profile: %v", errorValue)
		return
	}
	picture, errorValue := os.ReadFile(picturePath)
	if errorValue != nil {
		log.Printf("buzz agent profile: reading the default picture failed: %v", errorValue)
		return
	}
	published, errorValue := giveTheAgentADefaultPicture(ctx, messenger, picture)
	if errorValue != nil {
		log.Printf("buzz agent profile: %v", errorValue)
		return
	}
	if published {
		log.Printf("buzz agent profile: published with the default picture %s", picturePath)
	}
}

func (service *Service) agentProfileLacksAPicture(ctx context.Context) bool {
	if strings.TrimSpace(service.Configuration.AgentProfilePicturePath) == "" {
		return false
	}
	messenger, errorValue := service.relayAgentProfile()
	if errorValue != nil {
		return false
	}
	held, errorValue := messenger.heldProfile(ctx)
	if errorValue != nil {
		return true
	}
	return held.Picture == ""
}

type relayAgentProfile struct {
	service     *Service
	agentSecret string
	agentPubkey string
}

func (service *Service) relayAgentProfile() (relayAgentProfile, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return relayAgentProfile{}, errBuzzKeySeedMissing
	}
	agentSecret := buzzidentity.Secret(seed, buzzidentity.AgentSubject)
	agentPubkey, errorValue := buzzPublicKey(agentSecret)
	if errorValue != nil {
		return relayAgentProfile{}, errorValue
	}
	return relayAgentProfile{service: service, agentSecret: agentSecret, agentPubkey: agentPubkey}, nil
}

func (messenger relayAgentProfile) heldProfile(ctx context.Context) (buzzProfileContent, error) {
	relay, errorValue := messenger.service.buzzDatabase()
	if errorValue != nil {
		return buzzProfileContent{}, errorValue
	}
	profiles, errorValue := latestProfilesByPubkey(ctx, relay)
	if errorValue != nil {
		return buzzProfileContent{}, errorValue
	}
	return profiles[messenger.agentPubkey], nil
}

func (messenger relayAgentProfile) uploadPicture(ctx context.Context, content []byte, mimeType string) (string, error) {
	uploader := media.Uploader{HTTPBaseURL: messenger.service.buzzMediaOrigin()}
	blob, errorValue := uploader.Upload(ctx, messenger.agentSecret, content, mimeType)
	if errorValue != nil {
		return "", errorValue
	}
	return blob.URL, nil
}

func (messenger relayAgentProfile) publishProfile(ctx context.Context, displayName string, pictureURL string) error {
	return messenger.service.publishBuzzProfile(ctx, messenger.agentSecret, displayName, pictureURL)
}
