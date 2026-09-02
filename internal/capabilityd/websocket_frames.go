package capabilityd

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
)

func randomWebSocketKey() (string, error) {
	value := make([]byte, 16)
	_, errorValue := rand.Read(value)
	if errorValue != nil {
		return "", errorValue
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

func readWebSocketFrame(writer io.Writer, reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, errorValue := io.ReadFull(reader, header); errorValue != nil {
		return nil, errorValue
	}

	opcode := header[0] & 0x0f
	isMasked := header[1]&0x80 != 0
	payloadLength := uint64(header[1] & 0x7f)
	switch payloadLength {
	case 126:
		extendedLength := make([]byte, 2)
		if _, errorValue := io.ReadFull(reader, extendedLength); errorValue != nil {
			return nil, errorValue
		}
		payloadLength = uint64(binary.BigEndian.Uint16(extendedLength))
	case 127:
		extendedLength := make([]byte, 8)
		if _, errorValue := io.ReadFull(reader, extendedLength); errorValue != nil {
			return nil, errorValue
		}
		payloadLength = binary.BigEndian.Uint64(extendedLength)
	}

	var maskKey []byte
	if isMasked {
		maskKey = make([]byte, 4)
		if _, errorValue := io.ReadFull(reader, maskKey); errorValue != nil {
			return nil, errorValue
		}
	}

	payload := make([]byte, payloadLength)
	if _, errorValue := io.ReadFull(reader, payload); errorValue != nil {
		return nil, errorValue
	}
	if isMasked {
		for index := range payload {
			payload[index] ^= maskKey[index%4]
		}
	}

	switch opcode {
	case 0x1:
		return payload, nil
	case 0x8:
		return nil, errors.New("websocket close: " + string(payload))
	case 0x9:
		_ = writeWebSocketFrame(writer, 0xA, payload)
		return nil, nil
	case 0xA:
		return nil, nil
	default:
		return nil, nil
	}
}

func writeWebSocketTextFrame(writer io.Writer, payload []byte) error {
	return writeWebSocketFrame(writer, 0x1, payload)
}

func mustEncodePlatformHandle(handle platformHandle) string {
	value, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return ""
	}
	return value
}

func expectedWebSocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func writeWebSocketFrame(writer io.Writer, opcode byte, payload []byte) error {
	maskKey := make([]byte, 4)
	if _, errorValue := rand.Read(maskKey); errorValue != nil {
		return errorValue
	}

	header := []byte{0x80 | opcode}
	payloadLength := len(payload)
	switch {
	case payloadLength < 126:
		header = append(header, 0x80|byte(payloadLength))
	case payloadLength <= 65535:
		header = append(header, 0x80|126)
		lengthBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(lengthBytes, uint16(payloadLength))
		header = append(header, lengthBytes...)
	default:
		header = append(header, 0x80|127)
		lengthBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lengthBytes, uint64(payloadLength))
		header = append(header, lengthBytes...)
	}
	header = append(header, maskKey...)

	maskedPayload := make([]byte, payloadLength)
	for index, value := range payload {
		maskedPayload[index] = value ^ maskKey[index%4]
	}

	_, errorValue := writer.Write(append(header, maskedPayload...))
	return errorValue
}
