package admind

import (
	"context"
	"testing"
)

func TestAnAddressNobodyPresentedIsNotAskedAbout(t *testing.T) {
	service := &Service{}

	known, errorValue := service.ensurePersonFromDirectory(context.Background(), "   ", "")

	if known || errorValue != nil {
		t.Fatalf("an account presenting no address has nothing to look up, got known=%v error=%v", known, errorValue)
	}
}

func TestAHostWithNoDirectoryFailsInsteadOfAnsweringNo(t *testing.T) {
	service := &Service{}

	known, errorValue := service.ensurePersonFromDirectory(context.Background(), "이샘플@example.com", "이샘플")

	if errorValue == nil {
		t.Fatal("answering 'not a member' because this host cannot reach the directory refuses a colleague over a configuration gap")
	}
	if known {
		t.Fatal("a lookup that failed cannot report the person as known")
	}
}
