package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

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
	if errorValue := writeFileAtomically(policyPath, document, 0o600); errorValue != nil {
		return fmt.Errorf("writing the roster to %s failed: %w", policyPath, errorValue)
	}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/reload", nil, nil)
}
