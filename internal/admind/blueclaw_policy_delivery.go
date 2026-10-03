package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const deliveredPolicyMode = 0o644

func (service *Service) deliverBlueclawPolicy(ctx context.Context, policyDocument any) error {
	document, errorValue := json.MarshalIndent(policyDocument, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	policyPath := service.Configuration.BlueclawPolicyDeliveryPath
	if errorValue := replaceWholeFile(policyPath, document); errorValue != nil {
		return fmt.Errorf("writing the roster to %s failed: %w", policyPath, errorValue)
	}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/reload", nil, nil)
}

// The agent reads this file while admind rewrites it, so truncating it
// leaves a window where the roster is half a document.
func replaceWholeFile(path string, document []byte) error {
	replacement, errorValue := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if errorValue != nil {
		return errorValue
	}
	replacementPath := replacement.Name()
	defer os.Remove(replacementPath)
	if errorValue := writeAndClose(replacement, document); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Chmod(replacementPath, deliveredPolicyMode); errorValue != nil {
		return errorValue
	}
	return os.Rename(replacementPath, path)
}

func writeAndClose(file *os.File, document []byte) error {
	if _, errorValue := file.Write(document); errorValue != nil {
		_ = file.Close()
		return errorValue
	}
	if errorValue := file.Sync(); errorValue != nil {
		_ = file.Close()
		return errorValue
	}
	return file.Close()
}
