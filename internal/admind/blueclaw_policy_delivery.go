package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// deliveredPolicyMode matches what provisioning writes. The agent runs as its own user
// inside the guest and reads this file through the share, so a stricter mode makes the
// roster unreadable and refuses everybody.
const deliveredPolicyMode = 0o644

// deliverBlueclawPolicy writes the roster the agent reads and then tells it to re-read.
//
// The agent cannot write this file: on a device it arrives on a delivery share mounted
// read-only inside the guest, so that an agent which must never author its own roster
// cannot. Writing it is the host's job, which is this process.
func (service *Service) deliverBlueclawPolicy(ctx context.Context, policyDocument any) error {
	document, errorValue := json.MarshalIndent(policyDocument, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	policyPath := service.Configuration.BlueclawPolicyDeliveryPath
	if errorValue := rewriteInPlace(policyPath, document); errorValue != nil {
		return fmt.Errorf("writing the roster to %s failed: %w", policyPath, errorValue)
	}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/reload", nil, nil)
}

// rewriteInPlace keeps the file's identity. The agent reads this across a virtiofs share,
// and replacing the file by rename leaves the guest holding the inode it already opened,
// so the host writes a new roster and the agent keeps answering from the old one.
func rewriteInPlace(path string, document []byte) error {
	file, errorValue := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, deliveredPolicyMode)
	if errorValue != nil {
		return errorValue
	}
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
