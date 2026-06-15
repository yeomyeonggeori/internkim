package llmbackend

import "testing"

func TestNormalizeUsageCapturesCacheAndCost(t *testing.T) {
	raw := openAIUsage{
		PromptTokens:     194,
		CompletionTokens: 6,
		TotalTokens:      200,
		Cost:             0.0123,
	}
	raw.PromptTokensDetails.CachedTokens = 150
	raw.PromptTokensDetails.CacheWriteTokens = 40
	raw.CompletionTokensDetails.ReasoningTokens = 2
	raw.CostDetails.UpstreamInferenceCost = 0.0098

	usage := normalizeUsage(raw)

	if usage.CachedPromptTokens != 150 {
		t.Fatalf("expected cached prompt tokens, got %d", usage.CachedPromptTokens)
	}
	if usage.CacheWriteTokens != 40 {
		t.Fatalf("expected cache write tokens, got %d", usage.CacheWriteTokens)
	}
	if usage.ReasoningTokens != 2 {
		t.Fatalf("expected reasoning tokens, got %d", usage.ReasoningTokens)
	}
	if usage.CostUSD != 0.0123 {
		t.Fatalf("expected cost, got %v", usage.CostUSD)
	}
	if usage.UpstreamInferenceCost != 0.0098 {
		t.Fatalf("expected upstream inference cost, got %v", usage.UpstreamInferenceCost)
	}
}

func TestNormalizeUsageFallsBackToTokenSum(t *testing.T) {
	usage := normalizeUsage(openAIUsage{PromptTokens: 10, CompletionTokens: 5})
	if usage.TotalTokens != 15 {
		t.Fatalf("expected total token fallback, got %d", usage.TotalTokens)
	}
}
