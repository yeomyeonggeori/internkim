package centralplane

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func valuesTheCentralPlaneDeclares(t *testing.T, declarationName string) []string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.Join("..", "..", "supabase", "functions", "_shared", "member-vocabulary.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	declaration := regexp.MustCompile(declarationName + ` = \[([^\]]+)\]`).FindStringSubmatch(string(source))
	if declaration == nil {
		t.Fatalf("the central plane no longer declares %s as an array, so this test is reading the wrong file", declarationName)
	}
	values := []string{}
	for _, match := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(declaration[1], -1) {
		values = append(values, match[1])
	}
	if len(values) == 0 {
		t.Fatalf("%s names nothing", declarationName)
	}
	slices.Sort(values)
	return values
}

func TestTheDaemonKnowsTheRolesTheCentralPlaneDeclares(t *testing.T) {
	declared := valuesTheCentralPlaneDeclares(t, "memberRoles")

	known := MemberRoles()
	slices.Sort(known)
	if !slices.Equal(known, declared) {
		t.Fatalf("the daemon knows roles %v and the central plane declares %v", known, declared)
	}
}

func TestTheDaemonKnowsTheStatusesTheCentralPlaneDeclares(t *testing.T) {
	declared := valuesTheCentralPlaneDeclares(t, "memberStatuses")

	known := MemberStatuses()
	slices.Sort(known)
	if !slices.Equal(known, declared) {
		t.Fatalf("the daemon knows statuses %v and the central plane declares %v", known, declared)
	}
}

func TestAMemberIsHereUntilTheRecordSaysTheyLeft(t *testing.T) {
	for _, status := range MemberStatuses() {
		hasLeft := Member{Status: status}.HasLeftTheCompany()
		wantsToHaveLeft := status == MemberStatusDeparted || status == MemberStatusWithdrawn
		if hasLeft != wantsToHaveLeft {
			t.Fatalf("%s: has left = %v, want %v", status, hasLeft, wantsToHaveLeft)
		}
	}
}
