package admind

import (
	"fmt"
)


func defaultTaskSizeDefinitions() []taskSizeDefinition {
	return defaultTaskSizeDefinitionsForLocale("ko")
}

func defaultTaskSizeDefinitionsForLocale(locale string) []taskSizeDefinition {
	if normalizeAdminLocale(locale) == "en" {
		return []taskSizeDefinition{
			sizeDefinitionForLocale("en", "XS", 1, 1, "Trivial change", "Call / 10 minute meeting / handoff / tidy-up / scheduling", "Done in a moment"),
			sizeDefinitionForLocale("en", "S", 2, 2, "Low risk change with a narrow blast radius", "Half hour meeting / short draft / customer or partner reply / briefing", "Several fit in a day"),
			sizeDefinitionForLocale("en", "M", 3, 8, "Small feature / change with real impact", "Report / meeting under two hours / external meeting / cross-team alignment / document", "Takes a dedicated day"),
			sizeDefinitionForLocale("en", "L", 5, 16, "Mid-size feature / change touching many areas", "Important external meeting / mid-size research / proposal draft / policy change", "Takes about two days"),
			sizeDefinitionForLocale("en", "XL", 8, 32, "Large feature / complex change / external integration", "Restructuring / negotiation / long meeting / workshop", "Takes about a week"),
			sizeDefinitionForLocale("en", "XXL", 13, 128, "Milestone", "Partnership design / contract structure / service planning / policy overhaul", "Must be split into smaller items"),
		}
	}
	return []taskSizeDefinition{
		sizeDefinition("XS", 1, 1, "아주 사소한 변경", "전화 / 10분 회의 / 전달 / 정리 / 일정 조율", "잠깐이면 끝낼 것"),
		sizeDefinition("S", 2, 2, "난이도 낮고 영향 범위 좁은 변경", "30분 내외 회의 / 간단 문서 초안 / 고객 및 파트너 대응 / 브리핑", "하루 여러 번도 처리 가능한 것"),
		sizeDefinition("M", 3, 8, "소형 기능 추가 / 영향 있는 변경", "보고서 작성 / 2시간 이내 회의 / 외부 미팅 / 팀 간 조율 / 문서 작성", "하루 날 잡고 해야 할 것"),
		sizeDefinition("L", 5, 16, "중형 기능 추가 / 다수 영향 있는 변경", "중요 외부 미팅 / 중형 리서치 / 기획서 초안 / 정책 변경", "이틀은 걸릴 것"),
		sizeDefinition("XL", 8, 32, "대형 기능 추가 / 복잡한 변경 / 외부 연동", "재정비 / 협상 / 장시간 미팅 / 워크샵", "일주일은 걸릴 것"),
		sizeDefinition("XXL", 13, 128, "마일스톤", "파트너십 설계 / 계약 구조 설계 / 서비스 기획 / 정책 개편", "반드시 하위 항목으로 쪼갤 것"),
	}
}

func sizeDefinition(name string, distanceKM int, maxHours int, developmentExample string, otherExample string, note string) taskSizeDefinition {
	return sizeDefinitionForLocale("ko", name, distanceKM, maxHours, developmentExample, otherExample, note)
}

func sizeDefinitionForLocale(locale string, name string, distanceKM int, maxHours int, developmentExample string, otherExample string, note string) taskSizeDefinition {
	size := taskSizeDefinition{
		Name:               name,
		DistanceKM:         distanceKM,
		MaxHours:           maxHours,
		DevelopmentExample: developmentExample,
		OtherExample:       otherExample,
		Note:               note,
		Score:              distanceKM,
	}
	size.Label = taskSizeLabelForLocale(size, locale)
	return size
}


func taskSizeLabelForLocale(size taskSizeDefinition, locale string) string {
	if normalizeAdminLocale(locale) == "en" {
		return fmt.Sprintf("%dkm · max %dh", size.DistanceKM, size.MaxHours)
	}
	return fmt.Sprintf("%dkm · 최대 %dh", size.DistanceKM, size.MaxHours)
}

func containsTaskSize(values []taskSizeDefinition, target string) bool {
	for _, value := range values {
		if value.Name == target {
			return true
		}
	}
	return false
}

