package admind

import (
	"context"
	"testing"
)

func TestChangeNoticeKeepsDetailWhenWordingIsUnavailable(t *testing.T) {
	service := NewService(Configuration{})
	detail := "| 일시 | 일정 |\n|---|---|\n| 9월 2일 | 촬영 |"

	message := service.changeNoticeWording(context.Background(), []string{"일정: 촬영"}, detail)

	if message != detail {
		t.Fatalf("a notice must survive a failed wording call, got %q", message)
	}
}

func TestChangeNoticeSkipsWordingWithoutFacts(t *testing.T) {
	service := NewService(Configuration{})
	detail := "| 일시 | 일정 |"

	if message := service.changeNoticeWording(context.Background(), []string{"", "  "}, detail); message != detail {
		t.Fatalf("empty facts must not reach the model, got %q", message)
	}
}
