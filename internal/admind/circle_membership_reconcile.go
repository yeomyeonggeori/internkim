package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type circleReconcileReport struct {
	Email   string   `json:"email"`
	Circles []string `json:"circles"`
	Applied bool     `json:"applied"`
}

func (service *Service) handleCircleMembershipReconcile(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.reconcileCircleMembership(
		request.Context(),
		strings.TrimSpace(request.URL.Query().Get("email")),
		request.URL.Query().Get("apply") == "true",
	)
	if errorValue != nil {
		log.Printf("circle membership reconcile failed: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(report)
}

func (service *Service) reconcileCircleMembership(ctx context.Context, email string, shouldApply bool) (circleReconcileReport, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return circleReconcileReport{}, fmt.Errorf("this action names the person by email")
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return circleReconcileReport{}, errorValue
	}
	circles, isKnown := blueclawCirclesByEmail(policyDocument)[normalizedEmail]
	if !isKnown {
		return circleReconcileReport{}, fmt.Errorf("%s carries no circles here", normalizedEmail)
	}

	report := circleReconcileReport{Email: normalizedEmail, Circles: circles}
	if !shouldApply {
		return report, nil
	}
	if errorValue := service.syncMattermostUserCircleMemberships(ctx, adminUserMutation{
		Email:   normalizedEmail,
		Circles: circles,
		Role:    adminUserRoleMember,
	}); errorValue != nil {
		return report, errorValue
	}
	report.Applied = true
	log.Printf("circle memberships reconciled onto the messenger for %s: %v", normalizedEmail, circles)
	return report, nil
}
