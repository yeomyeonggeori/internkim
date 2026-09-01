package admind

import (
	"context"
	"testing"
)

func failOrganizationUserMutationCompletion(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.Exec(`
		CREATE TRIGGER fail_organization_user_mutation_completion
		BEFORE UPDATE OF active_mutations ON organization_people_cache_states
		WHEN NEW.active_mutations < OLD.active_mutations
		BEGIN
			SELECT RAISE(FAIL, 'forced organization user mutation completion failure');
		END
	`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func dropOrganizationUserMutationCompletionFailure(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`DROP TRIGGER fail_organization_user_mutation_completion`); errorValue != nil {
		t.Fatal(errorValue)
	}
}
