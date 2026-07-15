package admind

import "testing"

func TestFlowQuickTaskContentPrefersExplicitContent(t *testing.T) {
	content := flowQuickTaskContent(" 고객지원 분기 결산 누락 항목 확인 ", "분기 결산 누락 항목 확인")
	if content != "고객지원 분기 결산 누락 항목 확인" {
		t.Fatalf("content = %q", content)
	}
}

func TestFlowQuickTaskContentFallsBackToInferredContent(t *testing.T) {
	content := flowQuickTaskContent("  ", "분기 결산 누락 항목 확인")
	if content != "분기 결산 누락 항목 확인" {
		t.Fatalf("content = %q", content)
	}
}
