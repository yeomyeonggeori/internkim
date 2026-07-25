package admind

import (
	"encoding/hex"
	"errors"
	"strings"
)

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func encodeBuzzNsec(secretKeyHex string) (string, error) {
	secretKey, errorValue := hex.DecodeString(strings.TrimSpace(secretKeyHex))
	if errorValue != nil || len(secretKey) != 32 {
		return "", errors.New("secret key must be 32 hex-encoded bytes")
	}
	data := convertBitsEightToFive(secretKey)
	checksum := bech32Checksum("nsec", data)
	encoded := strings.Builder{}
	encoded.WriteString("nsec1")
	for _, value := range append(data, checksum...) {
		encoded.WriteByte(bech32Charset[value])
	}
	return encoded.String(), nil
}

func convertBitsEightToFive(data []byte) []byte {
	result := []byte{}
	accumulator := 0
	bits := 0
	for _, value := range data {
		accumulator = accumulator<<8 | int(value)
		bits += 8
		for bits >= 5 {
			bits -= 5
			result = append(result, byte(accumulator>>bits&31))
		}
	}
	if bits > 0 {
		result = append(result, byte(accumulator<<(5-bits)&31))
	}
	return result
}

func bech32Checksum(humanReadablePart string, data []byte) []byte {
	values := []byte{}
	for _, character := range humanReadablePart {
		values = append(values, byte(character)>>5)
	}
	values = append(values, 0)
	for _, character := range humanReadablePart {
		values = append(values, byte(character)&31)
	}
	values = append(values, data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	checksum := make([]byte, 6)
	for index := range checksum {
		checksum[index] = byte(polymod >> (5 * (5 - index)) & 31)
	}
	return checksum
}

func bech32Polymod(values []byte) int {
	generator := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	checksum := 1
	for _, value := range values {
		top := checksum >> 25
		checksum = (checksum&0x1ffffff)<<5 ^ int(value)
		for index := 0; index < 5; index++ {
			if (top>>index)&1 == 1 {
				checksum ^= generator[index]
			}
		}
	}
	return checksum
}
