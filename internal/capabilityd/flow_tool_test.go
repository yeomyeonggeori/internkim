package capabilityd

import "testing"

func TestMatchFlowMemberFindsNameInsidePrompt(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "lee", Email: "lee@example.com"},
		{ID: "iam", Name: "iam", Email: "iam@example.com"},
	}
	memberID := matchFlowMember("lee에게 10분 회의 추가해줘", members)
	if memberID != "lee" {
		t.Fatalf("memberID = %q", memberID)
	}
}
