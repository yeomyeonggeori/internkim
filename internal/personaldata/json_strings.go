package personaldata

import (
	"bytes"
	"encoding/json"
	"strings"
)

func (masker Masker) MaskJSON(document []byte, substitutions *Substitutions) ([]byte, bool) {
	if !json.Valid(document) {
		return document, false
	}
	return rewriteStringValues(document, mightHoldAValue, func(value string) string {
		return masker.MaskText(value, substitutions)
	}), true
}

func (substitutions *Substitutions) UnmaskJSON(document []byte) []byte {
	if !json.Valid(document) {
		return document
	}
	return rewriteStringValues(document, mightHoldAToken, substitutions.UnmaskText)
}

func mightHoldAValue(literal []byte) bool {
	return containsKeyMarker(literal) || hasEnoughDigitsForAValue(literal)
}

func hasEnoughDigitsForAValue(literal []byte) bool {
	for start := 0; start < len(literal); {
		if !isNumberCharacter(literal[start]) {
			start++
			continue
		}
		end := endOfNumberRun(literal, start)
		if digitCount(literal[start:end]) >= fewestDigitsInAValue {
			return true
		}
		start = end
	}
	return false
}

func mightHoldAToken(literal []byte) bool {
	return bytes.Contains(literal, []byte("{{"))
}

func rewriteStringValues(document []byte, mightChange func([]byte) bool, rewrite func(string) string) []byte {
	var rewritten []byte
	copiedUntil := 0
	for position := 0; position < len(document); position++ {
		if document[position] != '"' {
			continue
		}
		end := endOfString(document, position)
		replacement, isChanged := rewriteStringValue(document, position, end, mightChange, rewrite)
		if isChanged {
			if rewritten == nil {
				rewritten = make([]byte, 0, len(document)+len(document)/8)
			}
			rewritten = append(append(rewritten, document[copiedUntil:position]...), replacement...)
			copiedUntil = end
		}
		position = end - 1
	}
	if rewritten == nil {
		return document
	}
	return append(rewritten, document[copiedUntil:]...)
}

func rewriteStringValue(document []byte, start int, end int, mightChange func([]byte) bool, rewrite func(string) string) ([]byte, bool) {
	if !mightChange(document[start:end]) || isObjectKey(document, end) {
		return nil, false
	}
	var value string
	if json.Unmarshal(document[start:end], &value) != nil || strings.HasPrefix(value, "data:") {
		return nil, false
	}
	next := rewrite(value)
	if next == value {
		return nil, false
	}
	return encodeString(next), true
}

func endOfString(document []byte, start int) int {
	for position := start + 1; position < len(document); position++ {
		switch document[position] {
		case '\\':
			position++
		case '"':
			return position + 1
		}
	}
	return len(document)
}

func isObjectKey(document []byte, end int) bool {
	rest := bytes.TrimLeft(document[end:], " \t\r\n")
	return len(rest) > 0 && rest[0] == ':'
}

func encodeString(value string) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.Encode(value)
	return bytes.TrimRight(buffer.Bytes(), "\n")
}
