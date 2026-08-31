package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

var (
	errCircleReconcileNeedsAnEmail         = errors.New("this action names the person by email")
	errCircleReconcileDoesNotKnowThePerson = errors.New("the policy names nobody at this address")
)

type circleReconcileReport struct {
	Email   string   `json:"email"`
	Circles []string `json:"circles"`
	Applied bool     `json:"applied"`
}

func (service *Service) handleCircleMembershipReconcile(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	email := strings.TrimSpace(request.URL.Query().Get("email"))
	if email == "" {
		http.Error(responseWriter, errCircleReconcileNeedsAnEmail.Error(), http.StatusBadRequest)
		return
	}
	report, errorValue := service.reconcileCircleMembership(request.Context(), email, request.URL.Query().Get("apply") == "true")
	if errors.Is(errorValue, errCircleReconcileDoesNotKnowThePerson) {
		http.Error(responseWriter, errorValue.Error(), http.StatusNotFound)
		return
	}
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
		return circleReconcileReport{}, errCircleReconcileNeedsAnEmail
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return circleReconcileReport{}, errorValue
	}
	circles, isKnown := blueclawCirclesByEmail(policyDocument)[normalizedEmail]
	if !isKnown {
		return circleReconcileReport{}, fmt.Errorf("%w: %s", errCircleReconcileDoesNotKnowThePerson, normalizedEmail)
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
