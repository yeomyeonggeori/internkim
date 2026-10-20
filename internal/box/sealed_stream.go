package box

import (
	"bufio"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	sealedStreamVersion   = 1
	sealedStreamChunkSize = 64 * 1024
	sealedFrameLimit      = sealedStreamChunkSize + 1024
	backupSealInformation = "internkim backup"
)

var ErrSealedStreamCut = errors.New("the sealed stream ends before its last frame")

type sealedStreamHeader struct {
	Version   int    `json:"version"`
	Recipient string `json:"recipient"`
	Enc       string `json:"enc"`
}

func BackupChunkPurpose(companyID, chunkID string) SealPurpose {
	return SealPurpose{
		Information:    backupSealInformation,
		AdditionalData: additionalDataOf(companyID, chunkID, "backup-chunk"),
	}
}

type streamSealer struct {
	destination io.Writer
	sender      *hpke.Sender
	purpose     SealPurpose
	pending     []byte
	isClosed    bool
}

func SealStreamTo(destination io.Writer, recipientEncryptionKey string, purpose SealPurpose) (io.WriteCloser, error) {
	recipientBytes, errorValue := decodedKey(recipientEncryptionKey)
	if errorValue != nil {
		return nil, fmt.Errorf("the recipient key %s: %w", recipientEncryptionKey, errorValue)
	}
	recipientKey, errorValue := ecdh.X25519().NewPublicKey(recipientBytes)
	if errorValue != nil {
		return nil, fmt.Errorf("the recipient key %s: %w", recipientEncryptionKey, errorValue)
	}
	publicKey, errorValue := hpke.NewDHKEMPublicKey(recipientKey)
	if errorValue != nil {
		return nil, errorValue
	}
	enc, sender, errorValue := hpke.NewSender(publicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
	if errorValue != nil {
		return nil, errorValue
	}
	header, errorValue := json.Marshal(sealedStreamHeader{Version: sealedStreamVersion, Recipient: recipientEncryptionKey, Enc: encodedKey(enc)})
	if errorValue != nil {
		return nil, errorValue
	}
	if _, errorValue := destination.Write(append(header, '\n')); errorValue != nil {
		return nil, errorValue
	}
	return &streamSealer{destination: destination, sender: sender, purpose: purpose}, nil
}

func (sealer *streamSealer) Write(plaintext []byte) (int, error) {
	if sealer.isClosed {
		return 0, errors.New("writing to a sealed stream that was already closed")
	}
	sealer.pending = append(sealer.pending, plaintext...)
	for len(sealer.pending) >= sealedStreamChunkSize {
		if errorValue := sealer.writeFrame(sealer.pending[:sealedStreamChunkSize], chunkAdditionalData(sealer.purpose)); errorValue != nil {
			return 0, errorValue
		}
		sealer.pending = append([]byte{}, sealer.pending[sealedStreamChunkSize:]...)
	}
	return len(plaintext), nil
}

func (sealer *streamSealer) Close() error {
	if sealer.isClosed {
		return nil
	}
	sealer.isClosed = true
	if len(sealer.pending) > 0 {
		if errorValue := sealer.writeFrame(sealer.pending, chunkAdditionalData(sealer.purpose)); errorValue != nil {
			return errorValue
		}
		sealer.pending = nil
	}
	return sealer.writeFrame(nil, finalAdditionalData(sealer.purpose))
}

func (sealer *streamSealer) writeFrame(plaintext []byte, additionalData []byte) error {
	ciphertext, errorValue := sealer.sender.Seal(additionalData, plaintext)
	if errorValue != nil {
		return errorValue
	}
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(ciphertext)))
	if _, errorValue := sealer.destination.Write(length); errorValue != nil {
		return errorValue
	}
	_, errorValue = sealer.destination.Write(ciphertext)
	return errorValue
}

func chunkAdditionalData(purpose SealPurpose) []byte {
	return []byte(purpose.AdditionalData + "|chunk")
}

func finalAdditionalData(purpose SealPurpose) []byte {
	return []byte(purpose.AdditionalData + "|final")
}

type streamOpener struct {
	source    *bufio.Reader
	recipient *hpke.Recipient
	purpose   SealPurpose
	opened    []byte
	isDone    bool
}

func (keyring Keyring) OpenStream(source io.Reader, purpose SealPurpose) (io.Reader, error) {
	buffered := bufio.NewReader(source)
	headerLine, errorValue := buffered.ReadBytes('\n')
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed stream's header: %w", errorValue)
	}
	var header sealedStreamHeader
	if errorValue := json.Unmarshal(headerLine, &header); errorValue != nil {
		return nil, fmt.Errorf("the sealed stream's header: %w", errorValue)
	}
	if header.Version != sealedStreamVersion {
		return nil, fmt.Errorf("the stream is sealed in version %d, and this box opens version %d", header.Version, sealedStreamVersion)
	}
	privateKey, errorValue := keyring.privateKeyFor(header.Recipient)
	if errorValue != nil {
		return nil, errorValue
	}
	recipient, errorValue := recipientFor(privateKey, header.Enc, purpose)
	if errorValue != nil {
		return nil, errorValue
	}
	return &streamOpener{source: buffered, recipient: recipient, purpose: purpose}, nil
}

func (keyring Keyring) privateKeyFor(recipient string) (*ecdh.PrivateKey, error) {
	if recipient == keyring.Identity.EncryptionPublicKey() {
		return keyring.Identity.encryptionKey, nil
	}
	if keyring.companyKey != nil && encodedKey(keyring.companyKey.PublicKey().Bytes()) == recipient {
		return keyring.companyKey, nil
	}
	return nil, fmt.Errorf("the stream is sealed to the key %s: %w", recipient, ErrCompanyKeyNotHeld)
}

func recipientFor(privateKey *ecdh.PrivateKey, encodedEnc string, purpose SealPurpose) (*hpke.Recipient, error) {
	enc, errorValue := decodedKey(encodedEnc)
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed stream's encapsulated key: %w", errorValue)
	}
	recipientKey, errorValue := hpke.NewDHKEMPrivateKey(privateKey)
	if errorValue != nil {
		return nil, errorValue
	}
	return hpke.NewRecipient(enc, recipientKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
}

func (opener *streamOpener) Read(destination []byte) (int, error) {
	for len(opener.opened) == 0 {
		if opener.isDone {
			return 0, io.EOF
		}
		if errorValue := opener.openNextFrame(); errorValue != nil {
			return 0, errorValue
		}
	}
	read := copy(destination, opener.opened)
	opener.opened = opener.opened[read:]
	return read, nil
}

func (opener *streamOpener) openNextFrame() error {
	ciphertext, errorValue := opener.nextFrame()
	if errorValue != nil {
		return errorValue
	}
	if plaintext, errorValue := opener.recipient.Open(chunkAdditionalData(opener.purpose), ciphertext); errorValue == nil {
		opener.opened = plaintext
		return nil
	}
	if _, errorValue := opener.recipient.Open(finalAdditionalData(opener.purpose), ciphertext); errorValue != nil {
		return fmt.Errorf("a frame of the sealed stream was sealed for another purpose or altered on the way: %w", errorValue)
	}
	if _, errorValue := opener.source.ReadByte(); !errors.Is(errorValue, io.EOF) {
		return errors.New("the sealed stream carries bytes after its last frame")
	}
	opener.isDone = true
	return nil
}

func (opener *streamOpener) nextFrame() ([]byte, error) {
	length := make([]byte, 4)
	if _, errorValue := io.ReadFull(opener.source, length); errorValue != nil {
		return nil, fmt.Errorf("%w: %v", ErrSealedStreamCut, errorValue)
	}
	frameLength := binary.BigEndian.Uint32(length)
	if frameLength > sealedFrameLimit {
		return nil, fmt.Errorf("a frame of the sealed stream claims %d bytes, more than any this box writes", frameLength)
	}
	ciphertext := make([]byte, frameLength)
	if _, errorValue := io.ReadFull(opener.source, ciphertext); errorValue != nil {
		return nil, fmt.Errorf("%w: %v", ErrSealedStreamCut, errorValue)
	}
	return ciphertext, nil
}
