package box

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

var backupPurpose = BackupPurpose(sampleCompanyID, "internkim-backup-20261019T000000Z.tar")

func sealedStreamOf(t *testing.T, recipient Identity, plaintext []byte) []byte {
	t.Helper()
	var sealed bytes.Buffer
	writer, errorValue := SealStreamTo(&sealed, recipient.EncryptionPublicKey(), backupPurpose)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := writer.Write(plaintext); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return sealed.Bytes()
}

func randomBytes(t *testing.T, length int) []byte {
	t.Helper()
	plaintext := make([]byte, length)
	if _, errorValue := rand.Read(plaintext); errorValue != nil {
		t.Fatal(errorValue)
	}
	return plaintext
}

func openedStream(keyring Keyring, sealed []byte, purpose SealPurpose) ([]byte, error) {
	reader, errorValue := keyring.OpenStream(bytes.NewReader(sealed), purpose)
	if errorValue != nil {
		return nil, errorValue
	}
	return io.ReadAll(reader)
}

func TestABackupSealedToTheCompanyKeyOpensOnEveryBoxThatHoldsIt(t *testing.T) {
	holder := freshTestIdentity(t)
	handedTo := freshTestIdentity(t)
	for _, length := range []int{0, 1, sealedStreamChunkSize - 1, sealedStreamChunkSize, 3*sealedStreamChunkSize + 17} {
		plaintext := randomBytes(t, length)
		sealed := sealedStreamOf(t, holder, plaintext)

		for name, keyring := range map[string]Keyring{
			"the box whose key it is": {Identity: holder},
			"a box it was handed to":  {Identity: handedTo, companyKey: holder.encryptionKey},
		} {
			opened, errorValue := openedStream(keyring, sealed, backupPurpose)
			if errorValue != nil {
				t.Fatalf("%s, %d bytes: %v", name, length, errorValue)
			}
			if !bytes.Equal(opened, plaintext) {
				t.Fatalf("%s, %d bytes: opened something else", name, length)
			}
		}
	}
}

func TestABackupDoesNotOpenForAnotherBoxPurposeOrAfterTampering(t *testing.T) {
	holder := freshTestIdentity(t)
	sealed := sealedStreamOf(t, holder, randomBytes(t, 2*sealedStreamChunkSize+5))

	if _, errorValue := openedStream(Keyring{Identity: freshTestIdentity(t)}, sealed, backupPurpose); !errors.Is(errorValue, ErrCompanyKeyNotHeld) {
		t.Fatalf("a box without the key opened it: %v", errorValue)
	}
	if _, errorValue := openedStream(Keyring{Identity: holder}, sealed, BackupPurpose(sampleCompanyID, "another-backup.tar")); errorValue == nil {
		t.Fatal("a backup opened under another archive's name")
	}
	altered := append([]byte{}, sealed...)
	altered[len(altered)/2] ^= 1
	if _, errorValue := openedStream(Keyring{Identity: holder}, altered, backupPurpose); errorValue == nil {
		t.Fatal("an altered backup opened")
	}
}

func TestABackupCutShortIsRefusedRatherThanReadAsComplete(t *testing.T) {
	holder := freshTestIdentity(t)
	sealed := sealedStreamOf(t, holder, randomBytes(t, 2*sealedStreamChunkSize))
	firstFrameEnd := bytes.IndexByte(sealed, '\n') + 1 + 4 + sealedStreamChunkSize + 16

	for name, cut := range map[string][]byte{
		"after a whole frame":   sealed[:firstFrameEnd],
		"inside a frame":        sealed[:firstFrameEnd+100],
		"before the last frame": sealed[:len(sealed)-20],
	} {
		if _, errorValue := openedStream(Keyring{Identity: holder}, cut, backupPurpose); errorValue == nil {
			t.Fatalf("a backup cut %s opened as complete", name)
		}
	}
	if _, errorValue := openedStream(Keyring{Identity: holder}, append(append([]byte{}, sealed...), 0), backupPurpose); errorValue == nil {
		t.Fatal("a backup with bytes after its last frame opened")
	}
}
