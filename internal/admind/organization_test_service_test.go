package admind

import (
	"context"
	"testing"
)

// The organization tests need a service that answers for a company. The
// directory is the company's, so the service carries fleet credentials and each
// test stubs what the directory says.
func newLocalUsersTestService(t *testing.T) *Service {
	t.Helper()
	service := NewService(Configuration{
		BlueclawBaseURL:            "http://blueclaw.local",
		BlueclawPolicyDeliveryPath: t.TempDir() + "/policy.json",
		APIBaseURL:                 "https://api.example.test",
		FleetIDPath:                writeTestFile(t, "dc719d8e"),
		FleetSecretPath:            writeTestFile(t, "secret-value"),
		StateDirectory:             t.TempDir(),
	})
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) {
		return nil, nil
	}
	return service
}
