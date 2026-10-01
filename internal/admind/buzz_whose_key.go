package admind

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

const buzzIdentityVersionsToTry = 8

type keyOwnerAnswer struct {
	Pubkey    string `json:"pubkey"`
	Owner     string `json:"owner,omitempty"`
	Version   int    `json:"version,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Addresses int    `json:"addressesTried"`
	Found     bool   `json:"found"`
}

func (service *Service) handleBuzzWhoseKey(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	pubkey := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("pubkey")))
	if pubkey == "" {
		http.Error(responseWriter, "this action names the key it is asking about", http.StatusBadRequest)
		return
	}
	answer, errorValue := service.whoseBuzzKey(request.Context(), pubkey)
	if errorValue != nil {
		log.Printf("whose buzz key failed: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(answer)
}

// Every key this device issues is derived from the seed and a subject, so the
// only way back from a key to a person is to derive every subject again and
// compare. A key nothing here derives was made somewhere else.
func (service *Service) whoseBuzzKey(ctx context.Context, pubkey string) (keyOwnerAnswer, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return keyOwnerAnswer{}, errors.New("this device names no buzz key seed")
	}
	answer := keyOwnerAnswer{Pubkey: pubkey}
	for _, subject := range []string{buzzidentity.BootstrapSubject, buzzidentity.AgentSubject} {
		derived, errorValue := buzzPublicKey(buzzidentity.Secret(seed, subject))
		if errorValue != nil {
			return answer, errorValue
		}
		answer.Addresses++
		if derived == pubkey {
			answer.Found = true
			answer.Owner = "this device"
			answer.Subject = subject
			return answer, nil
		}
	}
	for _, email := range service.everyAddressThisDeviceKnows(ctx) {
		answer.Addresses++
		for version := 1; version <= buzzIdentityVersionsToTry; version++ {
			subject := versionedSubject(email, version)
			derived, errorValue := buzzPublicKey(buzzidentity.Secret(seed, subject))
			if errorValue != nil {
				return answer, errorValue
			}
			if derived == pubkey {
				answer.Found = true
				answer.Owner = email
				answer.Version = version
				answer.Subject = subject
				return answer, nil
			}
		}
	}
	return answer, nil
}

func (service *Service) everyAddressThisDeviceKnows(ctx context.Context) []string {
	seen := map[string]bool{}
	addresses := []string{}
	add := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		addresses = append(addresses, email)
	}
	for _, email := range service.allMemberEmails(ctx) {
		add(email)
	}
	if client := service.centralPlane(); client != nil {
		if members, errorValue := client.Members(ctx); errorValue == nil {
			for _, member := range members {
				add(member.Email)
			}
		}
	}
	return addresses
}
