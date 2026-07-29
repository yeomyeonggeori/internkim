package admind

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"hash"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

const mattermostPasswordHashFileName = "mattermost-password-hashes.json"

// Existing employees sign in with the password already held in Mattermost's
// user table as a bcrypt hash. Exporting those hashes into admind's own state
// lets login be verified on the device, so Mattermost the application can be
// retired without locking anyone out. The hashes never leave the device and the
// live Mattermost login stays as a fallback while it runs.
func (service *Service) mattermostPasswordHashPath() string {
	return filepath.Join(service.Configuration.StateDirectory, mattermostPasswordHashFileName)
}

func (service *Service) startMattermostPasswordHashSync(ctx context.Context) {
	if strings.TrimSpace(service.Configuration.MattermostBaseURL) == "" {
		return
	}
	go func() {
		service.syncMattermostPasswordHashes(ctx)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.syncMattermostPasswordHashes(ctx)
			}
		}
	}()
}

func (service *Service) syncMattermostPasswordHashes(ctx context.Context) {
	query := `psql -tA -c "select lower(email), password from users where deleteat=0" mattermost`
	output, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", query)
	if errorValue != nil {
		log.Printf("mattermost password hash sync failed: %v", errorValue)
		return
	}
	hashes := parseMattermostPasswordHashes(string(output))
	if len(hashes) == 0 {
		return
	}
	blob, errorValue := json.Marshal(hashes)
	if errorValue != nil {
		return
	}
	if errorValue := writeFileAtomically(service.mattermostPasswordHashPath(), blob, 0o600); errorValue != nil {
		log.Printf("mattermost password hash store write failed: %v", errorValue)
	}
}

func parseMattermostPasswordHashes(output string) map[string]string {
	hashes := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(fields) != 2 || fields[0] == "" || !isSupportedPasswordHash(fields[1]) {
			continue
		}
		hashes[fields[0]] = fields[1]
	}
	return hashes
}

func isSupportedPasswordHash(hashValue string) bool {
	return strings.HasPrefix(hashValue, "$2") || strings.HasPrefix(hashValue, "$pbkdf2$")
}

func (service *Service) verifyLocalMattermostPassword(email string, password string) bool {
	blob, errorValue := os.ReadFile(service.mattermostPasswordHashPath())
	if errorValue != nil {
		return false
	}
	var hashes map[string]string
	if errorValue := json.Unmarshal(blob, &hashes); errorValue != nil {
		return false
	}
	storedHash := hashes[strings.ToLower(strings.TrimSpace(email))]
	if storedHash == "" {
		return false
	}
	return verifyMattermostPasswordHash(storedHash, password)
}

// Mattermost stores passwords either as bcrypt ($2…) or, on current releases,
// as a PBKDF2 PHC string: $pbkdf2$f=SHA256,w=600000,l=32$<salt>$<derived>, with
// salt and derived key in unpadded standard base64.
func verifyMattermostPasswordHash(storedHash string, password string) bool {
	if strings.HasPrefix(storedHash, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	if strings.HasPrefix(storedHash, "$pbkdf2$") {
		return verifyPBKDF2PasswordHash(storedHash, password)
	}
	return false
}

func verifyPBKDF2PasswordHash(storedHash string, password string) bool {
	segments := strings.Split(storedHash, "$")
	if len(segments) != 5 || segments[1] != "pbkdf2" {
		return false
	}
	parameters := parsePBKDF2Parameters(segments[2])
	iterations, iterationsError := strconv.Atoi(parameters["w"])
	keyLength, keyLengthError := strconv.Atoi(parameters["l"])
	hashConstructor, hashKnown := pbkdf2HashConstructor(parameters["f"])
	if iterationsError != nil || keyLengthError != nil || !hashKnown || iterations <= 0 || keyLength <= 0 {
		return false
	}
	salt, saltError := base64.RawStdEncoding.DecodeString(segments[3])
	expected, expectedError := base64.RawStdEncoding.DecodeString(segments[4])
	if saltError != nil || expectedError != nil {
		return false
	}
	derived := pbkdf2.Key([]byte(password), salt, iterations, keyLength, hashConstructor)
	return subtle.ConstantTimeCompare(derived, expected) == 1
}

func parsePBKDF2Parameters(specification string) map[string]string {
	parameters := map[string]string{}
	for _, pair := range strings.Split(specification, ",") {
		key, value, found := strings.Cut(pair, "=")
		if found {
			parameters[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return parameters
}

func pbkdf2HashConstructor(name string) (func() hash.Hash, bool) {
	switch strings.ToUpper(name) {
	case "SHA256":
		return sha256.New, true
	case "SHA512":
		return sha512.New, true
	case "SHA1":
		return sha1.New, true
	default:
		return nil, false
	}
}
