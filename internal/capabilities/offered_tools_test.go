package capabilities

import (
	"sort"
	"testing"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestTheAgentIsOfferedEveryToolTheRecordOrTheCompanyAnswers(t *testing.T) {
	offeredToolNames := map[string]bool{}
	for _, descriptor := range DefaultToolDescriptors() {
		offeredToolNames[descriptor.Name] = true
	}
	absentToolNames := []string{}
	for _, descriptor := range capabilityprotocol.GeneratedToolDescriptorSet() {
		if descriptor.AnsweredBy == capabilityprotocol.AnsweredByLocal {
			continue
		}
		if !offeredToolNames[descriptor.Name] {
			absentToolNames = append(absentToolNames, descriptor.Name)
		}
	}
	sort.Strings(absentToolNames)
	if len(absentToolNames) > 0 {
		t.Fatalf("the catalog says the record or the company answers %v, and this set offers the agent none of them", absentToolNames)
	}
}
