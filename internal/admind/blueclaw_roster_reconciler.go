package admind

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// bootstrapPersonEmail is the account a device is born with. A host serving a real
// company holds people beside it; holding it alone means the projection was lost.
const bootstrapPersonEmail = "admin@example.test"

// ReconcileBlueclawRoster makes the agent's people match the company's account
// directory. The directory is the owner and this is a projection, so a person added
// centrally reaches the agent here rather than through anyone typing on the host.
//
// It reports rather than repairs one case: a roster that holds only the bootstrap
// account while the directory has members. The repair is the same either way, but
// that state means the projection was lost rather than never built, and it stayed
// invisible last time until an employee was refused.
func (service *Service) ReconcileBlueclawRoster(ctx context.Context) error {
	client := service.centralPlane()
	if client == nil {
		return fmt.Errorf("the central plane is not configured, so this host has no directory to derive its roster from")
	}
	members, errorValue := client.Members(ctx)
	if errorValue != nil {
		return errorValue
	}
	activeMembers := activeMembersOf(members)
	if len(activeMembers) == 0 {
		return fmt.Errorf("the company directory returned no active member, which is never a reason to empty this host's roster")
	}

	knownEmails, errorValue := service.blueclawPolicyEmails(ctx)
	if errorValue != nil {
		return errorValue
	}
	if holdsOnlyBootstrapAccount(knownEmails) {
		log.Printf("blueclaw.roster.lost: the agent holds only %s while the directory has %d active members; rebuilding", bootstrapPersonEmail, len(activeMembers))
	}

	missingMembers := membersMissingFrom(activeMembers, knownEmails)
	for _, member := range missingMembers {
		if errorValue := service.upsertBlueclawPerson(ctx, member.MemberID, member.Email, member.Name, member.Role, nil, nil); errorValue != nil {
			return fmt.Errorf("projecting %s onto the agent failed: %w", member.Email, errorValue)
		}
	}
	log.Printf("blueclaw.roster.reconciled: %d active members, %d added", len(activeMembers), len(missingMembers))
	return nil
}

func membersMissingFrom(activeMembers []centralplane.Member, knownEmails map[string]bool) []centralplane.Member {
	missingMembers := []centralplane.Member{}
	for _, member := range activeMembers {
		if knownEmails[strings.ToLower(strings.TrimSpace(member.Email))] {
			continue
		}
		missingMembers = append(missingMembers, member)
	}
	return missingMembers
}

func activeMembersOf(members []centralplane.Member) []centralplane.Member {
	activeMembers := []centralplane.Member{}
	for _, member := range members {
		if member.IsActive() && strings.TrimSpace(member.Email) != "" {
			activeMembers = append(activeMembers, member)
		}
	}
	return activeMembers
}

func holdsOnlyBootstrapAccount(knownEmails map[string]bool) bool {
	return len(knownEmails) == 1 && knownEmails[bootstrapPersonEmail]
}

func (service *Service) blueclawPolicyEmails(ctx context.Context) (map[string]bool, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, "GET", "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	knownEmails := map[string]bool{}
	people, _ := policyDocument["people"].([]any)
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		emails, _ := person["emails"].([]any)
		for _, emailValue := range emails {
			email, isString := emailValue.(string)
			if !isString {
				continue
			}
			knownEmails[strings.ToLower(strings.TrimSpace(email))] = true
		}
	}
	return knownEmails, nil
}

// rosterReconcileInterval is how often a host re-derives its roster. A person added
// centrally reaches the agent within it, and a projection lost to anything the host
// did not notice is rebuilt within it too.
const rosterReconcileInterval = 10 * time.Minute

func (service *Service) keepBlueclawRosterDerived(ctx context.Context) {
	if service.centralPlane() == nil {
		return
	}
	service.reconcileBlueclawRosterOnce(ctx)

	ticker := time.NewTicker(rosterReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.reconcileBlueclawRosterOnce(ctx)
		}
	}
}

func (service *Service) reconcileBlueclawRosterOnce(ctx context.Context) {
	if errorValue := service.ReconcileBlueclawRoster(ctx); errorValue != nil {
		log.Printf("blueclaw.roster.reconcile_failed: %v", errorValue)
	}
}
