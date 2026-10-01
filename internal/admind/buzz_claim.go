package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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

const buzzCredentialSweepInterval = 2 * time.Minute

// Recording ran when the directory said it changed and once at startup, so a
// signal that never arrived left somebody without a key until this process was
// restarted: they could speak, because the roster reconciles on its own clock,
// and the agent could not tell who they were. It reconciles on a clock too now.
func (service *Service) startBuzzCredentialSweep(ctx context.Context) {
	go func() {
		log.Printf("buzz credentials at startup: %s", service.recordBuzzCredentials(ctx))
		ticker := time.NewTicker(buzzCredentialSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				recording := service.recordBuzzCredentials(ctx)
				if len(recording.Refusals) > 0 {
					log.Printf("buzz credential sweep: %s", recording)
				}
				service.withinASweepBudget(ctx, service.showOutWhoeverNobodyNames)
			}
		}
	}()
}

// The people the record already names get their key without anybody pressing a
// button, because the messenger they use every day is the thing that is broken.
func (service *Service) recordBuzzCredentials(ctx context.Context) buzzCredentialRecording {
	client := service.centralPlane()
	if client == nil {
		return buzzCredentialRecording{Refusals: []string{"this host has no company directory configured"}}
	}
	members, errorValue := client.Members(ctx)
	if errorValue != nil {
		return buzzCredentialRecording{Refusals: []string{fmt.Sprintf("the company did not say who works here: %v", errorValue)}}
	}
	recording := buzzCredentialRecording{}
	for _, member := range members {
		email := strings.ToLower(strings.TrimSpace(member.Email))
		if email == "" || member.HasLeftTheCompany() {
			recording.Skipped++
			continue
		}
		secretHex, errorValue := service.personBuzzSecret(ctx, email)
		if errorValue != nil {
			recording.refuse(email, "the key could not be derived", errorValue)
			continue
		}
		publicHex, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil {
			recording.refuse(email, "the derived key has no public half", errorValue)
			continue
		}
		if errorValue := client.KeepMessengerCredential(ctx, member.MemberID, buzzCredentialKind, publicHex, secretHex); errorValue != nil {
			recording.refuse(email, "the company would not keep it", errorValue)
			continue
		}
		recording.Kept++
	}
	return recording
}

// A count that cannot say why is the thing this whole day was about. Every
// refusal carries the address it was for and what refused it.
type buzzCredentialRecording struct {
	Kept     int
	Skipped  int
	Refusals []string
}

func (recording *buzzCredentialRecording) refuse(email string, because string, errorValue error) {
	recording.Refusals = append(recording.Refusals, fmt.Sprintf("%s: %s: %v", email, because, errorValue))
}

func (recording buzzCredentialRecording) String() string {
	summary := fmt.Sprintf("kept %d, skipped %d, refused %d", recording.Kept, recording.Skipped, len(recording.Refusals))
	if len(recording.Refusals) == 0 {
		return summary
	}
	return summary + "\n" + strings.Join(recording.Refusals, "\n")
}
