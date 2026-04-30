package browser

import (
	"strings"
	"testing"
)

func TestIsCaptchaBlockedDetectsEnglishSignatures(t *testing.T) {
	testCases := []string{
		"Verify you're not a robot",
		"To continue, please verify you are human",
		"Our systems have detected unusual traffic from your computer network",
		"This page checks to see if it's really you sending the requests, not a robot. reCAPTCHA",
		"Robot Check\nPlease confirm you're a human.",
		"Before you continue to Google",
	}
	for _, snapshotText := range testCases {
		isBlocked, signature := IsCaptchaBlocked(snapshotText)
		if !isBlocked {
			t.Errorf("expected captcha blocked for %q, got false", snapshotText)
			continue
		}
		if signature == "" {
			t.Errorf("expected matched signature for %q, got empty", snapshotText)
		}
	}
}

func TestIsCaptchaBlockedDetectsKoreanSignatures(t *testing.T) {
	testCases := []string{
		"비정상적인 트래픽이 감지되었습니다",
		"사람인지 확인하세요",
		"본인이 로봇이 아닙니다 체크박스를 선택해주세요",
		"자동화된 요청이 차단되었습니다",
	}
	for _, snapshotText := range testCases {
		isBlocked, _ := IsCaptchaBlocked(snapshotText)
		if !isBlocked {
			t.Errorf("expected captcha blocked for %q, got false", snapshotText)
		}
	}
}

func TestIsCaptchaBlockedSkipsLongLegitimateContent(t *testing.T) {
	longLegitimateContent := strings.Repeat(
		"Wikipedia article about CAPTCHA: A CAPTCHA is a type of challenge-response test... ",
		200,
	)
	isBlocked, _ := IsCaptchaBlocked(longLegitimateContent)
	if isBlocked {
		t.Errorf("expected long legitimate content to skip captcha detection, got blocked")
	}
}

func TestIsCaptchaBlockedReturnsFalseForEmptyText(t *testing.T) {
	isBlocked, signature := IsCaptchaBlocked("")
	if isBlocked || signature != "" {
		t.Errorf("expected empty text to return false, got blocked=%v signature=%q", isBlocked, signature)
	}
}

func TestIsCaptchaBlockedReturnsFalseForNormalContent(t *testing.T) {
	normalContent := "Welcome to the homepage. Today's weather in Seoul is sunny with a high of 22°C."
	isBlocked, _ := IsCaptchaBlocked(normalContent)
	if isBlocked {
		t.Errorf("expected normal content to not be blocked, got blocked")
	}
}
