package localfleet

import "testing"

func fleetForTest(t *testing.T) Service {
	t.Helper()
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repository",
		ExecutablePath:     "/repository/internkim",
		StateRootPath:      "/state",
		VirtualMachineName: "internkim-local-fleet",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return service
}
