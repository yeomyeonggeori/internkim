package admind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestATaskWriteOnADeviceWithNoCompanyStaysOnTheDevice(t *testing.T) {
	service := &Service{}
	request := httptest.NewRequest(http.MethodPost, quickTaskPath, nil)

	_, answered, errorValue := service.addTaskThroughTheRecord(request, Task{Content: "분기 보고서 초안"}, "", nil)
	if answered || errorValue != nil {
		t.Fatalf("a device that names no company keeps its own board, got answered=%v error=%v", answered, errorValue)
	}
}

func TestACompanyBoardRefusesAWriteItCannotAttribute(t *testing.T) {
	service := serviceWithACompanyForTest(t)
	request := httptest.NewRequest(http.MethodPost, quickTaskPath, nil)

	_, answered, errorValue := service.addTaskThroughTheRecord(request, Task{Content: "분기 보고서 초안"}, "", nil)
	if !answered {
		t.Fatal("a company device answers for its board rather than writing beside it")
	}
	if !errors.Is(errorValue, errTaskWriterUnnamed) {
		t.Fatalf("a write nobody is named on is refused, got %v", errorValue)
	}
}

