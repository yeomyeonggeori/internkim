package llmbackend

import (
	"encoding/json"
)

func ValidateStructuredJSON(content string) bool {
	var parsedContent any
	return json.Unmarshal([]byte(content), &parsedContent) == nil
}
