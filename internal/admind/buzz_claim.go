package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

type buzzClaimResponse struct {
	SecretHex string `json:"secretHex"`
	PublicHex string `json:"publicHex"`
}

// handleBuzzClaim hands the caller their own Buzz secret key once, gated on the
// Cloudflare Access verified email. The browser wraps it with the user's
// passkey or password and stores only the sealed blob (POST /agent/api/buzz-
// vault); the raw key never persists server-side beyond the derivation seed.
// The gate is Cloudflare Access — an authentication layer, not a messaging
// platform — so the Buzz identity stays independent of any messenger client.
func (service *Service) handleBuzzClaim(responseWriter http.ResponseWriter, request *http.Request) {
	// Deriving a key changes nothing, and the relay bridge a company browser
	// comes through asks with GET. The secret is in the answer, never the URL.
	if request.Method != http.MethodPost && request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	actorEmail := service.actorEmailAllowingAssertedRequester(request)
	if actorEmail == "" {
		actorEmail = service.authenticatedCallerEmail(request)
	}
	if actorEmail == "" {
		http.Error(responseWriter, "authentication is required", http.StatusUnauthorized)
		return
	}
	secretHex, errorValue := service.personBuzzSecret(request.Context(), actorEmail)
	if errors.Is(errorValue, errBuzzKeySeedMissing) {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "buzz_claim_failed", http.StatusInternalServerError)
		return
	}
	publicHex, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		http.Error(responseWriter, "buzz_claim_failed", http.StatusInternalServerError)
		return
	}
	service.rememberBuzzCredential(request.Context(), actorEmail, secretHex, publicHex)
	service.writeJSON(responseWriter, buzzClaimResponse{SecretHex: secretHex, PublicHex: publicHex})
}

// The machine derives this key from a seed nobody else holds, so the record has
// to be told. Otherwise the web messenger, which never sees a seed, keeps being
// handed whatever credential this person had before the messenger changed.
//
// Handing the key back is what the caller asked for; recording it is not, so a
// company that will not take it does not turn this into a failure.
func (service *Service) rememberBuzzCredential(ctx context.Context, actorEmail string, secretHex string, publicHex string) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	member, found, errorValue := client.MemberByEmail(ctx, actorEmail)
	if errorValue != nil || !found {
		log.Printf("the buzz key for %s was not recorded: the company does not name that address", actorEmail)
		return
	}
	if errorValue := client.KeepMessengerCredential(ctx, member.MemberID, buzzCredentialKind, publicHex, secretHex); errorValue != nil {
		log.Printf("the buzz key for %s was not recorded: %v", actorEmail, errorValue)
		return
	}
	log.Printf("the buzz key for %s is now the credential the record holds", actorEmail)
}

// chatd asks for this kind by name (chatd/src/personal/buzz.ts).
var buzzCredentialKind = capabilityprotocol.MessengerIdentityCredentialKind()

func (service *Service) recordMessengerKeys(ctx context.Context, client *centralplane.Client, members []centralplane.Member) error {
	var refusals []error
	for _, member := range members {
		email := normalizedRosterEmail(member.Email)
		if email == "" || member.HasLeftTheCompany() {
			continue
		}
		if errorValue := service.recordMessengerKey(ctx, client, member.MemberID, email); errorValue != nil {
			refusals = append(refusals, fmt.Errorf("the messenger key for %s was not recorded: %w", email, errorValue))
		}
	}
	return errors.Join(refusals...)
}

func (service *Service) recordMessengerKey(ctx context.Context, client *centralplane.Client, memberID string, email string) error {
	secretHex, errorValue := service.personBuzzSecret(ctx, email)
	if errorValue != nil {
		return errorValue
	}
	publicHex, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		return errorValue
	}
	return client.KeepMessengerCredential(ctx, memberID, buzzCredentialKind, publicHex, secretHex)
}
