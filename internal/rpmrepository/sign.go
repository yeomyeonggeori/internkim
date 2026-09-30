package rpmrepository

import (
	"bytes"
	"fmt"
)

const (
	signatureTagHeaderSignature = 268
	signatureTagBodySignature   = 1002
)

var signatureTagsReplaced = map[int]bool{
	267:                         true,
	signatureTagHeaderSignature: true,
	1003:                        true,
	signatureTagBodySignature:   true,
	1005:                        true,
}

type packageLayout struct {
	lead      []byte
	signature header
	header    []byte
	payload   []byte
}

func splitPackage(contents []byte) (packageLayout, header, error) {
	if len(contents) < leadLength || !bytes.Equal(contents[:4], leadMagic) {
		return packageLayout{}, header{}, fmt.Errorf("not an rpm: no lead magic")
	}
	signature, errorValue := readHeader(contents, leadLength)
	if errorValue != nil {
		return packageLayout{}, header{}, fmt.Errorf("signature header: %w", errorValue)
	}
	headerStart := leadLength + signature.length
	headerStart += (signatureAlignment - headerStart%signatureAlignment) % signatureAlignment
	mainHeader, errorValue := readHeader(contents, headerStart)
	if errorValue != nil {
		return packageLayout{}, header{}, fmt.Errorf("package header: %w", errorValue)
	}
	headerEnd := headerStart + mainHeader.length
	return packageLayout{
		lead:      contents[:leadLength],
		signature: signature,
		header:    contents[headerStart:headerEnd],
		payload:   contents[headerEnd:],
	}, mainHeader, nil
}

// Sign replaces the signature tags of an rpm with the archive key's, leaving
// the lead, the header and the payload as they were. nfpm can sign while it
// builds, but `release packages` builds with no key in reach and the repository
// step is the one that holds it; a header signature covers the header bytes and
// a body signature the header and payload, so neither depends on who built it.
func Sign(contents []byte, signer interface {
	DetachSignBinary(document []byte) ([]byte, error)
}) ([]byte, error) {
	layout, _, errorValue := splitPackage(contents)
	if errorValue != nil {
		return nil, errorValue
	}
	entries := map[int]headerEntry{}
	for tag, entry := range layout.signature.entries {
		if !signatureTagsReplaced[tag] {
			entries[tag] = entry
		}
	}
	headerSignature, errorValue := signer.DetachSignBinary(layout.header)
	if errorValue != nil {
		return nil, fmt.Errorf("sign the rpm header: %w", errorValue)
	}
	entries[signatureTagHeaderSignature] = headerEntry{tag: signatureTagHeaderSignature, kind: typeBinary, count: len(headerSignature), data: headerSignature}

	headerAndPayload := append(append([]byte{}, layout.header...), layout.payload...)
	bodySignature, errorValue := signer.DetachSignBinary(headerAndPayload)
	if errorValue != nil {
		return nil, fmt.Errorf("sign the rpm payload: %w", errorValue)
	}
	entries[signatureTagBodySignature] = headerEntry{tag: signatureTagBodySignature, kind: typeBinary, count: len(bodySignature), data: bodySignature}

	encoded := encodeRegionHeader(signatureRegionTag, entries)
	var signed bytes.Buffer
	signed.Write(layout.lead)
	signed.Write(encoded)
	signed.Write(make([]byte, (signatureAlignment-len(encoded)%signatureAlignment)%signatureAlignment))
	signed.Write(layout.header)
	signed.Write(layout.payload)
	return signed.Bytes(), nil
}

func IsSigned(contents []byte) bool {
	layout, _, errorValue := splitPackage(contents)
	if errorValue != nil {
		return false
	}
	_, hasHeaderSignature := layout.signature.entries[signatureTagHeaderSignature]
	_, hasBodySignature := layout.signature.entries[signatureTagBodySignature]
	return hasHeaderSignature && hasBodySignature
}
