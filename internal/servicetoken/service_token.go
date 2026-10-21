package servicetoken

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func Authorize(request *http.Request, tokenPath string) error {
	trimmedTokenPath := strings.TrimSpace(tokenPath)
	if trimmedTokenPath == "" {
		return nil
	}
	token, errorValue := os.ReadFile(trimmedTokenPath)
	if errorValue != nil {
		return fmt.Errorf("the service token at %s could not be read: %w", trimmedTokenPath, errorValue)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	return nil
}
