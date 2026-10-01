package mail

import (
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

func connectedBoxIn(t *testing.T, companyID string) (string, box.Identity) {
	t.Helper()
	stateDirectoryPath := t.TempDir()
	identity, errorValue := box.LoadOrCreateIdentity(filepath.Join(stateDirectoryPath, "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(stateDirectoryPath, "company"), []byte(companyID+"\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return stateDirectoryPath, identity
}

func sealedTo(t *testing.T, identity box.Identity, password string, purpose box.SealPurpose) *box.SealedSecret {
	t.Helper()
	keyBytes, errorValue := base64.RawURLEncoding.DecodeString(identity.EncryptionPublicKey())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	ecdhKey, errorValue := ecdh.X25519().NewPublicKey(keyBytes)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	publicKey, errorValue := hpke.NewDHKEMPublicKey(ecdhKey)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	enc, sender, errorValue := hpke.NewSender(publicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	ciphertext, errorValue := sender.Seal([]byte(purpose.AdditionalData), []byte(password))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return &box.SealedSecret{
		Version:    1,
		Recipient:  identity.EncryptionPublicKey(),
		Enc:        base64.RawURLEncoding.EncodeToString(enc),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}
}

func aSealedAccount(t *testing.T, identity box.Identity) Account {
	t.Helper()
	account := aConfiguredAccount()
	account.IMAPPort, account.IMAPSecurity, account.SMTPPort, account.SMTPSecurity = 993, "tls", 587, "starttls"
	account.SealedIMAPPassword = sealedTo(t, identity, "imap-secret", box.MailPasswordPurpose("company-a", "member-a", "IMAPPassword",
		box.MailConnection{Host: account.IMAPHost, Port: account.IMAPPort, Security: account.IMAPSecurity, Username: account.IMAPUsername}))
	account.SealedSMTPPassword = sealedTo(t, identity, "smtp-secret", box.MailPasswordPurpose("company-a", "member-a", "SMTPPassword",
		box.MailConnection{Host: account.SMTPHost, Port: account.SMTPPort, Security: account.SMTPSecurity, Username: account.SMTPUsername}))
	account.IMAPPassword, account.SMTPPassword = "", ""
	return account
}

func TestTheBoxOpensEachPasswordForTheServerItWasSealedFor(t *testing.T) {
	stateDirectoryPath, identity := connectedBoxIn(t, "company-a")

	opened, errorValue := BoxPasswords(stateDirectoryPath)(aSealedAccount(t, identity), "member-a")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opened.IMAPPassword != "imap-secret" || opened.SMTPPassword != "smtp-secret" {
		t.Fatalf("opened %q and %q", opened.IMAPPassword, opened.SMTPPassword)
	}
}

func TestAPasswordIsNotOpenedForAServerItWasNotSealedFor(t *testing.T) {
	stateDirectoryPath, identity := connectedBoxIn(t, "company-a")
	for name, rewrite := range map[string]func(*Account){
		"imap host":     func(account *Account) { account.IMAPHost = "imap.attacker.test" },
		"imap port":     func(account *Account) { account.IMAPPort = 143 },
		"imap security": func(account *Account) { account.IMAPSecurity = "none" },
		"imap username": func(account *Account) { account.IMAPUsername = "other" },
		"smtp host":     func(account *Account) { account.SMTPHost = "smtp.attacker.test" },
	} {
		account := aSealedAccount(t, identity)
		rewrite(&account)
		if _, errorValue := BoxPasswords(stateDirectoryPath)(account, "member-a"); errorValue == nil {
			t.Fatalf("a password opened after its %s was rewritten", name)
		}
	}
	if _, errorValue := BoxPasswords(stateDirectoryPath)(aSealedAccount(t, identity), "member-b"); errorValue == nil {
		t.Fatal("a password opened for another member")
	}
}
