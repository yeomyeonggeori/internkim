package blueclaw

import (
	"fmt"
	"strings"
)

func NormalizeMaximumModelTier(modelTier string) (string, error) {
	normalizedModelTier := strings.ToLower(strings.TrimSpace(modelTier))
	if normalizedModelTier == "" {
		return "", nil
	}
	for _, supportedModelTier := range []string{"xlow", "low", "medium", "high", "xhigh", "max"} {
		if normalizedModelTier == supportedModelTier {
			return normalizedModelTier, nil
		}
	}
	return "", fmt.Errorf("maximum model tier must be xlow, low, medium, high, xhigh, or max: %s", modelTier)
}
