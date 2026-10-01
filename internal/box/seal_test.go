package box

import (
	"crypto/hpke"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var imapConnection = MailConnection{Host: "imap.example.test", Port: 993, Security: "tls", Username: "sample"}

var mailPurpose = MailPasswordPurpose("company-a", "member-a", "IMAPPassword", imapConnection)

func sealSecretTo(t *testing.T, recipient Identity, plaintext string, purpose SealPurpose) SealedSecret {
	t.Helper()
	publicKey, errorValue := hpke.NewDHKEMPublicKey(recipient.encryptionKey.PublicKey())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	enc, sender, errorValue := hpke.NewSender(publicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	ciphertext, errorValue := sender.Seal([]byte(purpose.AdditionalData), []byte(plaintext))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return SealedSecret{
		Version:    sealedSecretVersion,
		Recipient:  recipient.EncryptionPublicKey(),
		Enc:        encodedKey(enc),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}
}

func freshTestIdentity(t *testing.T) Identity {
	t.Helper()
	identity, errorValue := freshIdentity()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return identity
}

func TestTheBoxOpensASecretSealedToItForThatOwner(t *testing.T) {
	identity := freshTestIdentity(t)

	opened, errorValue := identity.OpenSecret(sealSecretTo(t, identity, "imap-secret", mailPurpose), mailPurpose)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opened != "imap-secret" {
		t.Fatalf("opened %q", opened)
	}
}

func TestASecretSealedForAnotherOwnerDoesNotOpen(t *testing.T) {
	identity := freshTestIdentity(t)
	sealed := sealSecretTo(t, identity, "imap-secret", mailPurpose)

	for _, asked := range []SealPurpose{
		MailPasswordPurpose("company-a", "member-b", "IMAPPassword", imapConnection),
		MailPasswordPurpose("company-b", "member-a", "IMAPPassword", imapConnection),
		MailPasswordPurpose("company-a", "member-a", "SMTPPassword", imapConnection),
		MailPasswordPurpose("company-a", "member-a", "IMAPPassword", MailConnection{Host: "imap.attacker.test", Port: 993, Security: "tls", Username: "sample"}),
		MailPasswordPurpose("company-a", "member-a", "IMAPPassword", MailConnection{Host: "imap.example.test", Port: 143, Security: "tls", Username: "sample"}),
		MailPasswordPurpose("company-a", "member-a", "IMAPPassword", MailConnection{Host: "imap.example.test", Port: 993, Security: "none", Username: "sample"}),
		MailPasswordPurpose("company-a", "member-a", "IMAPPassword", MailConnection{Host: "imap.example.test", Port: 993, Security: "tls", Username: "other"}),
		MailPasswordPurpose("company-a", "member-a", "IMAPPassword", MailConnection{Host: "imap.example.test|sample", Port: 993, Security: "tls", Username: ""}),
		{Information: modelKeySealInformation, AdditionalData: mailPurpose.AdditionalData},
	} {
		if _, errorValue := identity.OpenSecret(sealed, asked); errorValue == nil {
			t.Fatalf("a secret sealed for %+v opened for %+v", mailPurpose, asked)
		}
	}
}

func TestASecretSealedToAnotherBoxDoesNotOpen(t *testing.T) {
	sealed := sealSecretTo(t, freshTestIdentity(t), "imap-secret", mailPurpose)
	otherBox := freshTestIdentity(t)

	if _, errorValue := otherBox.OpenSecret(sealed, mailPurpose); errorValue == nil {
		t.Fatal("a box the secret was not sealed to opened it")
	}
	sealed.Recipient = otherBox.EncryptionPublicKey()
	if _, errorValue := otherBox.OpenSecret(sealed, mailPurpose); errorValue == nil {
		t.Fatal("renaming the recipient let another box open the secret")
	}
}

func TestASecretOfAnUnknownVersionIsRefused(t *testing.T) {
	identity := freshTestIdentity(t)
	sealed := sealSecretTo(t, identity, "imap-secret", mailPurpose)
	sealed.Version = sealedSecretVersion + 1

	if _, errorValue := identity.OpenSecret(sealed, mailPurpose); errorValue == nil {
		t.Fatal("a secret of a version this box does not know was opened")
	}
}

func TestAComputerWithoutABoxKeySaysSo(t *testing.T) {
	_, errorValue := LoadConnected(t.TempDir())

	if !errors.Is(errorValue, ErrNoBoxKey) {
		t.Fatalf("answered %v", errorValue)
	}
}

func TestAConnectedBoxNamesItsCompany(t *testing.T) {
	stateDirectoryPath := t.TempDir()
	identity, errorValue := LoadOrCreateIdentity(identityPathIn(stateDirectoryPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(stateDirectoryPath, companyMarkerFileName), []byte("company-a\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	connected, errorValue := LoadConnected(stateDirectoryPath)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if connected.CompanyID != "company-a" || connected.Identity.EncryptionPublicKey() != identity.EncryptionPublicKey() {
		t.Fatalf("loaded %s for %s", connected.Identity.EncryptionPublicKey(), connected.CompanyID)
	}
}

type mailSealingVector struct {
	Password     string `json:"password"`
	BoxSecretKey string `json:"boxSecretKey"`
	CompanyID    string `json:"companyID"`
	MemberID     string `json:"memberID"`
	Field        string `json:"field"`
	Connection   struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Security string `json:"security"`
		Username string `json:"username"`
	} `json:"connection"`
	Sealed SealedSecret `json:"sealed"`
}

func (vector mailSealingVector) connection() MailConnection {
	return MailConnection(vector.Connection)
}

func readMailSealingVector(t *testing.T) mailSealingVector {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("testdata", "sealed-mail-password.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var vector mailSealingVector
	if errorValue := json.Unmarshal(document, &vector); errorValue != nil {
		t.Fatal(errorValue)
	}
	return vector
}

func TestBoxOpensTheMailPasswordTheWebAppSealed(t *testing.T) {
	vector := readMailSealingVector(t)
	identity := identityWithEncryptionSeed(t, vector.BoxSecretKey)

	opened, errorValue := identity.OpenSecret(vector.Sealed, MailPasswordPurpose(vector.CompanyID, vector.MemberID, vector.Field, vector.connection()))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opened != vector.Password {
		t.Fatalf("opened %q, sealed %q", opened, vector.Password)
	}
}

func TestTheMailPasswordTheWebAppSealedOpensForNoOtherMemberOrServer(t *testing.T) {
	vector := readMailSealingVector(t)
	identity := identityWithEncryptionSeed(t, vector.BoxSecretKey)

	if _, errorValue := identity.OpenSecret(vector.Sealed, MailPasswordPurpose(vector.CompanyID, vector.CompanyID, vector.Field, vector.connection())); errorValue == nil {
		t.Fatal("a password sealed for one member opened for another")
	}
	moved := vector.connection()
	moved.Host = "imap.attacker.test"
	if _, errorValue := identity.OpenSecret(vector.Sealed, MailPasswordPurpose(vector.CompanyID, vector.MemberID, vector.Field, moved)); errorValue == nil {
		t.Fatal("a password sealed for one server opened for another")
	}
}

type rfc9180Vector struct {
	Info       string `json:"info"`
	SkRm       string `json:"skRm"`
	PkRm       string `json:"pkRm"`
	Enc        string `json:"enc"`
	Aad        string `json:"aad"`
	Plaintext  string `json:"pt"`
	Ciphertext string `json:"ct"`
}

func hexBytes(t *testing.T, encoded string) []byte {
	t.Helper()
	decoded, errorValue := hex.DecodeString(encoded)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return decoded
}

func TestTheBoxOpensTheRFC9180VectorForItsSuite(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("testdata", "hpke-rfc9180-base-x25519-sha256-aes256gcm.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var vector rfc9180Vector
	if errorValue := json.Unmarshal(document, &vector); errorValue != nil {
		t.Fatal(errorValue)
	}
	identity := identityWithEncryptionSeed(t, encodedKey(hexBytes(t, vector.SkRm)))
	if identity.EncryptionPublicKey() != encodedKey(hexBytes(t, vector.PkRm)) {
		t.Fatal("the vector's recipient key is not the one its secret key derives")
	}

	opened, errorValue := identity.OpenSecret(SealedSecret{
		Version:    sealedSecretVersion,
		Recipient:  identity.EncryptionPublicKey(),
		Enc:        encodedKey(hexBytes(t, vector.Enc)),
		Ciphertext: base64.RawURLEncoding.EncodeToString(hexBytes(t, vector.Ciphertext)),
	}, SealPurpose{Information: string(hexBytes(t, vector.Info)), AdditionalData: string(hexBytes(t, vector.Aad))})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opened != string(hexBytes(t, vector.Plaintext)) {
		t.Fatalf("opened %q", opened)
	}
}

type modelKeyFixture struct {
	ModelKey     string         `json:"modelKey"`
	BoxSecretKey string         `json:"boxSecretKey"`
	CompanyID    string         `json:"companyID"`
	Sealed       SealedModelKey `json:"sealed"`
}

func TestBoxOpensTheModelKeyTheBrowserSealed(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("testdata", "sealed-model-key.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fixture modelKeyFixture
	if errorValue := json.Unmarshal(document, &fixture); errorValue != nil {
		t.Fatal(errorValue)
	}

	identity := identityWithEncryptionSeed(t, fixture.BoxSecretKey)
	modelKey, errorValue := identity.OpenModelKey(fixture.Sealed, fixture.CompanyID)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if modelKey != fixture.ModelKey {
		t.Fatalf("opened %q, sealed %q", modelKey, fixture.ModelKey)
	}
	if _, errorValue := freshTestIdentity(t).OpenModelKey(fixture.Sealed, fixture.CompanyID); errorValue == nil {
		t.Fatal("a box the model key was not sealed to opened it")
	}
	if _, errorValue := identity.OpenModelKey(fixture.Sealed, sampleCompanyID); errorValue == nil {
		t.Fatal("a model key sealed for one company opened for another")
	}
}

func TestAModelKeyDoesNotOpenAsAMailPassword(t *testing.T) {
	identity := freshTestIdentity(t)
	sealed := sealSecretTo(t, identity, "sk-or-v1-sample", ModelKeyPurpose("company-a", identity.EncryptionPublicKey()))

	if _, errorValue := identity.OpenSecret(sealed, mailPurpose); errorValue == nil {
		t.Fatal("a model key opened as a mail password")
	}
}
