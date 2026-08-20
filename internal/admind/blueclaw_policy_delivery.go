package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	if errorValue := writeFileAtomically(policyPath, document, deliveredPolicyMode); errorValue != nil {
		return fmt.Errorf("writing the roster to %s failed: %w", policyPath, errorValue)
	}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/reload", nil, nil)
}
