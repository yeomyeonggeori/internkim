package admind

import "testing"

func TestPreferExplicitFlowTaskValue(t *testing.T) {
	title := preferExplicitFlowTaskValue(" 고객지원 분기 결산 누락 항목 확인 ", "분기 결산 누락 항목 확인")
	if title != "고객지원 분기 결산 누락 항목 확인" {
		t.Fatalf("title = %q", title)
	}
	endDate := preferExplicitFlowTaskValue(" 2026-07-18 ", "2026-07-17")
	if endDate != "2026-07-18" {
		t.Fatalf("endDate = %q", endDate)
	}
}

func TestPreferExplicitFlowTaskValueFallsBackToInference(t *testing.T) {
	value := preferExplicitFlowTaskValue("  ", "추론 값")
	if value != "추론 값" {
		t.Fatalf("value = %q", value)
	}
}
