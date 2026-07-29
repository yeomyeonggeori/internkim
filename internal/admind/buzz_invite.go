package admind

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const buzzInviteLifetime = 72 * time.Hour
const buzzMemberPollInterval = 30 * time.Second

type buzzInvite struct {
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	IdentityPubkey string    `json:"identityPubkey,omitempty"`
	ClaimedPubkey  string    `json:"claimedPubkey,omitempty"`
}

type buzzInviteState struct {
	Invites            []buzzInvite      `json:"invites"`
	Links              map[string]string `json:"links"`
	KnownMemberPubkeys []string          `json:"knownMemberPubkeys"`
}

type buzzInviteStore struct {
	mutex sync.Mutex
	path  string
	state buzzInviteState
}

type buzzMemberRecord struct {
	Pubkey  string
	AddedBy string
}

func (service *Service) buzzInviteEnabled() bool {
	return strings.TrimSpace(service.Configuration.BuzzInviteKeyPath) != "" &&
		strings.TrimSpace(service.Configuration.BuzzCommunityID) != ""
}

func (service *Service) buzzStore() *buzzInviteStore {
	service.buzzInviteStoreOnce.Do(func() {
		store := &buzzInviteStore{path: filepath.Join(service.Configuration.StateDirectory, "buzz-invites.json")}
		store.load()
		service.buzzInviteStore = store
	})
	return service.buzzInviteStore
}

func (store *buzzInviteStore) load() {
	store.state = buzzInviteState{Links: map[string]string{}}
	document, errorValue := os.ReadFile(store.path)
	if errorValue != nil {
		return
	}
	var state buzzInviteState
	if json.Unmarshal(document, &state) == nil {
		if state.Links == nil {
			state.Links = map[string]string{}
		}
		store.state = state
	}
}

func (store *buzzInviteStore) save() {
	document, errorValue := json.MarshalIndent(store.state, "", "  ")
	if errorValue != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(store.path), 0o755)
	_ = os.WriteFile(store.path, document, 0o600)
}

func (service *Service) handleBuzz(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "buzz access requires staff login", http.StatusForbidden)
		return
	}
	if !service.buzzInviteEnabled() {
		http.Error(responseWriter, "buzz invites are not configured", http.StatusNotImplemented)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/buzz/api")
	switch {
	case request.Method == http.MethodPost && path == "/invites":
		service.handleBuzzInviteCreate(responseWriter, request)
	case request.Method == http.MethodGet && path == "/invites":
		service.handleBuzzInviteList(responseWriter)
	case request.Method == http.MethodPost && path == "/links":
		service.handleBuzzLinkCreate(responseWriter, request)
	case request.Method == http.MethodGet && path == "/config":
		service.writeJSON(responseWriter, map[string]string{
			"relayURL": service.Configuration.BuzzRelayURL,
			"deepLink": "buzz://connect?relay=" + url.QueryEscape(service.Configuration.BuzzRelayURL),
		})
	default:
		http.NotFound(responseWriter, request)
	}
}

type buzzInviteCreateRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (service *Service) handleBuzzInviteCreate(responseWriter http.ResponseWriter, request *http.Request) {
	var payload buzzInviteCreateRequest
	if errorValue := decodeOptionalJSONBody(request.Body, &payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(payload.Name)
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if name == "" || email == "" || !strings.Contains(email, "@") {
		http.Error(responseWriter, "name and a valid email are required", http.StatusBadRequest)
		return
	}
	code, expiresAt, errorValue := service.mintBuzzInvite()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	secretHex, identityError := service.personBuzzSecret(request.Context(), email)
	var identityPubkey string
	if identityError == nil {
		identityPubkey, identityError = buzzPublicKey(secretHex)
	}
	if identityError != nil {
		log.Printf("buzz identity derivation failed for %s: %v", email, identityError)
	}
	store := service.buzzStore()
	store.mutex.Lock()
	invite := buzzInvite{
		Code:      code,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	}
	if identityError == nil {
		invite.IdentityPubkey = identityPubkey
		store.state.Links[identityPubkey] = email
	}
	store.state.Invites = append(store.state.Invites, invite)
	store.save()
	store.mutex.Unlock()
	if identityError == nil {
		service.writeBuzzAccountLinksFile()
	}
	response := map[string]string{
		"inviteURL": service.buzzInviteURL(code),
		"code":      code,
		"expiresAt": expiresAt.Format(time.RFC3339),
	}
	if identityError == nil {
		response["identityPubkey"] = identityPubkey
		if nsec, nsecError := encodeBuzzNsec(secretHex); nsecError == nil {
			response["identityPrivateKey"] = nsec
		} else {
			response["identityPrivateKey"] = secretHex
		}
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) buzzInviteURL(code string) string {
	landingBaseURL := strings.TrimRight(service.Configuration.BuzzLandingBaseURL, "/")
	if landingBaseURL == "" {
		return "buzz://join?relay=" + url.QueryEscape(service.Configuration.BuzzRelayURL) + "&code=" + url.QueryEscape(code)
	}
	return landingBaseURL + "/invite.html?relay=" + url.QueryEscape(service.Configuration.BuzzRelayURL) + "&code=" + url.QueryEscape(code)
}

func (service *Service) handleBuzzInviteList(responseWriter http.ResponseWriter) {
	store := service.buzzStore()
	store.mutex.Lock()
	defer store.mutex.Unlock()
	unlinkedPubkeys := []string{}
	for _, pubkey := range store.state.KnownMemberPubkeys {
		if _, isLinked := store.state.Links[pubkey]; !isLinked {
			unlinkedPubkeys = append(unlinkedPubkeys, pubkey)
		}
	}
	service.writeJSON(responseWriter, map[string]any{
		"invites":         store.state.Invites,
		"links":           store.state.Links,
		"unlinkedPubkeys": unlinkedPubkeys,
	})
}

type buzzLinkCreateRequest struct {
	Pubkey string `json:"pubkey"`
	Email  string `json:"email"`
}

func (service *Service) handleBuzzLinkCreate(responseWriter http.ResponseWriter, request *http.Request) {
	var payload buzzLinkCreateRequest
	if errorValue := decodeOptionalJSONBody(request.Body, &payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	pubkey := strings.ToLower(strings.TrimSpace(payload.Pubkey))
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if len(pubkey) != 64 || !isLowercaseHexString(pubkey) || email == "" {
		http.Error(responseWriter, "a 64-character hex pubkey and email are required", http.StatusBadRequest)
		return
	}
	store := service.buzzStore()
	store.mutex.Lock()
	store.state.Links[pubkey] = email
	store.save()
	store.mutex.Unlock()
	service.writeBuzzAccountLinksFile()
	service.writeJSON(responseWriter, map[string]string{"pubkey": pubkey, "email": email})
}

func (service *Service) mintBuzzInvite() (string, time.Time, error) {
	inviteKey, errorValue := service.readBuzzInviteKey()
	if errorValue != nil {
		return "", time.Time{}, errorValue
	}
	nonce := make([]byte, 16)
	if _, errorValue := rand.Read(nonce); errorValue != nil {
		return "", time.Time{}, errorValue
	}
	expiresAt := time.Now().UTC().Add(buzzInviteLifetime)
	payload, errorValue := json.Marshal(map[string]any{
		"c": service.Configuration.BuzzCommunityID,
		"r": "member",
		"e": expiresAt.Unix(),
		"n": base64.RawURLEncoding.EncodeToString(nonce),
	})
	if errorValue != nil {
		return "", time.Time{}, errorValue
	}
	mac := hmac.New(sha256.New, inviteKey)
	mac.Write(payload)
	code := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return code, expiresAt, nil
}

func (service *Service) readBuzzInviteKey() ([]byte, error) {
	document, errorValue := os.ReadFile(service.Configuration.BuzzInviteKeyPath)
	if errorValue != nil {
		return nil, errorValue
	}
	return hex.DecodeString(strings.TrimSpace(string(document)))
}

func (service *Service) startBuzzMemberLinker(ctx context.Context) {
	if !service.buzzInviteEnabled() || strings.TrimSpace(service.Configuration.BuzzAdminCommandPath) == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(buzzMemberPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.linkClaimedBuzzMembers(ctx)
			}
		}
	}()
}

func (service *Service) startBuzzAccountLinkSync(ctx context.Context) {
	if strings.TrimSpace(service.Configuration.BuzzAccountLinksPath) == "" || service.buzzKeySeed() == "" {
		return
	}
	go func() {
		service.linkDeterministicBuzzPeople(ctx)
		ticker := time.NewTicker(buzzMemberPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.linkDeterministicBuzzPeople(ctx)
			}
		}
	}()
}

func (service *Service) linkDeterministicBuzzPeople(ctx context.Context) {
	if service.buzzKeySeed() == "" {
		return
	}
	derivedLinks := map[string]string{}
	for _, record := range service.blueclawPolicyUserRecords(ctx) {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if email == "" {
			continue
		}
		secretHex := service.buzzSecretForEmail(ctx, email)
		if secretHex == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil {
			continue
		}
		derivedLinks[pubkey] = email
	}
	if len(derivedLinks) == 0 {
		return
	}
	store := service.buzzStore()
	store.mutex.Lock()
	changed := false
	for pubkey, email := range derivedLinks {
		if store.state.Links[pubkey] == email {
			continue
		}
		store.state.Links[pubkey] = email
		changed = true
	}
	if changed {
		store.save()
	}
	store.mutex.Unlock()
	if changed {
		service.writeBuzzAccountLinksFile()
	}
}

func (service *Service) linkClaimedBuzzMembers(ctx context.Context) {
	members, errorValue := service.listBuzzMembers(ctx)
	if errorValue != nil {
		return
	}
	store := service.buzzStore()
	store.mutex.Lock()
	changed := false
	isBaselineSnapshot := len(store.state.KnownMemberPubkeys) == 0
	known := map[string]bool{}
	for _, pubkey := range store.state.KnownMemberPubkeys {
		known[pubkey] = true
	}
	for _, member := range members {
		if known[member.Pubkey] {
			continue
		}
		known[member.Pubkey] = true
		store.state.KnownMemberPubkeys = append(store.state.KnownMemberPubkeys, member.Pubkey)
		changed = true
		if isBaselineSnapshot || member.AddedBy != "invite" {
			continue
		}
		if invite := inviteByIdentityPubkey(store.state.Invites, member.Pubkey); invite != nil {
			invite.ClaimedPubkey = member.Pubkey
			continue
		}
		if invite := singleOutstandingInvite(store.state.Invites); invite != nil {
			invite.ClaimedPubkey = member.Pubkey
			store.state.Links[member.Pubkey] = invite.Email
		}
	}
	if changed {
		store.save()
	}
	store.mutex.Unlock()
	if changed {
		service.writeBuzzAccountLinksFile()
	}
}

func inviteByIdentityPubkey(invites []buzzInvite, pubkey string) *buzzInvite {
	for index := range invites {
		if invites[index].IdentityPubkey == pubkey {
			return &invites[index]
		}
	}
	return nil
}

func singleOutstandingInvite(invites []buzzInvite) *buzzInvite {
	var outstanding *buzzInvite
	now := time.Now().UTC()
	for index := range invites {
		invite := &invites[index]
		if invite.ClaimedPubkey != "" || invite.IdentityPubkey != "" || now.After(invite.ExpiresAt) {
			continue
		}
		if outstanding != nil {
			return nil
		}
		outstanding = invite
	}
	return outstanding
}

func (service *Service) listBuzzMembers(ctx context.Context) ([]buzzMemberRecord, error) {
	command := exec.CommandContext(ctx, service.Configuration.BuzzAdminCommandPath, "list-members")
	command.Env = append(os.Environ(), "DATABASE_URL="+service.Configuration.BuzzDatabaseURL)
	output, errorValue := command.Output()
	if errorValue != nil {
		return nil, errorValue
	}
	return parseBuzzMemberList(string(output)), nil
}

func parseBuzzMemberList(output string) []buzzMemberRecord {
	records := []buzzMemberRecord{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pubkey := strings.ToLower(fields[0])
		if len(pubkey) != 64 || !isLowercaseHexString(pubkey) {
			continue
		}
		records = append(records, buzzMemberRecord{Pubkey: pubkey, AddedBy: fields[2]})
	}
	return records
}

func isLowercaseHexString(value string) bool {
	for _, character := range value {
		isDigit := character >= '0' && character <= '9'
		isHexLetter := character >= 'a' && character <= 'f'
		if !isDigit && !isHexLetter {
			return false
		}
	}
	return true
}

func (service *Service) writeBuzzAccountLinksFile() {
	linksPath := strings.TrimSpace(service.Configuration.BuzzAccountLinksPath)
	if linksPath == "" {
		return
	}
	store := service.buzzStore()
	store.mutex.Lock()
	document, errorValue := json.MarshalIndent(store.state.Links, "", "  ")
	store.mutex.Unlock()
	if errorValue != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(linksPath), 0o755)
	_ = os.WriteFile(linksPath, document, 0o644)
}
