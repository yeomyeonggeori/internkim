package box

import (
	"errors"
	"fmt"
	"os"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

func keepTheSessionInTheConnection(connectionPath, accessToken string) error {
	if _, errorValue := os.Stat(connectionPath); errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	if errorValue := companyhost.KeepAgentKey(connectionPath, accessToken); errorValue != nil {
		return fmt.Errorf("keeping the renewed session in %s: %w", connectionPath, errorValue)
	}
	return nil
}
