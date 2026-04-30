package browser

import "strings"

const captchaSnapshotMaxLength = 4096

var captchaSignatures = []string{
	"verify you're not a robot",
	"verify you are human",
	"unusual traffic from your computer network",
	"automated queries",
	"before you continue to google",
	"recaptcha",
	"robot check",
	"비정상적인 트래픽",
	"사람인지 확인",
	"로봇이 아닙니다",
	"자동화된 요청",
}

func IsCaptchaBlocked(snapshotText string) (bool, string) {
	trimmedText := strings.TrimSpace(snapshotText)
	if trimmedText == "" {
		return false, ""
	}
	if len(trimmedText) > captchaSnapshotMaxLength {
		return false, ""
	}
	lowercaseText := strings.ToLower(trimmedText)
	for _, signature := range captchaSignatures {
		if strings.Contains(lowercaseText, signature) {
			return true, signature
		}
	}
	return false, ""
}
