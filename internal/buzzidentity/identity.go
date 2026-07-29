// Package buzzidentity is the single source of truth for deterministic Buzz
// identity and channel derivation. The importer (cmd/buzz-migrate), admind, and
// the chatd mirror all derive the same pubkey for the same person and the same
// channel id for the same source channel, so imported history and live traffic
// share one identity. The TypeScript mirror mirrors these exact formulas
// (chatd src/mirror/identity.ts) and is anchored by a cross-language test.
//
// The seed is a critical root secret: losing it makes every derived key and
// imported message unrecoverable. Persist and back it up; never pass it as a
// throwaway env var. See docs/buzz-identity-seed.md.
package buzzidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Secret derives a person's Buzz secret key from a seed and their email.
// Email is lowercased and trimmed so the same person always resolves to the
// same key regardless of casing or surrounding whitespace.
const BootstrapSubject = "__bootstrap__"

func Secret(seed, email string) string {
	digest := sha256.Sum256([]byte(seed + "|secret|" + strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(digest[:])
}

// ChannelID derives the canonical Buzz channel id (UUID-shaped) for a source
// platform channel from a seed and the channel's native id.
func ChannelID(seed, externalChannelID string) string {
	digest := sha256.Sum256([]byte(seed + "|channel|" + externalChannelID))
	return fmt.Sprintf("%x-%x-%x-%x-%x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}
