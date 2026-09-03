package capabilityd

import (
	"reflect"
	"sort"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func toolNamesAnsweredBy(answerer string) []string {
	names := []string{}
	for _, descriptor := range capabilities.DefaultToolDescriptors() {
		if descriptor.AnsweredBy == answerer {
			names = append(names, descriptor.CanonicalName)
		}
	}
	sort.Strings(names)
	return names
}

func TestEveryRegisteredToolNamesItsAnswerer(t *testing.T) {
	answerers := map[string]bool{
		capabilityprotocol.AnsweredByRecord:  true,
		capabilityprotocol.AnsweredByCompany: true,
		capabilityprotocol.AnsweredByLocal:   true,
	}
	for _, descriptor := range capabilities.DefaultToolDescriptors() {
		if !answerers[descriptor.AnsweredBy] {
			t.Errorf("%s answeredBy is %q", descriptor.CanonicalName, descriptor.AnsweredBy)
		}
	}
}

func TestTheRecordsToolsAreCarriedToTheRecord(t *testing.T) {
	carrier := reflect.ValueOf(Service.invokeRecordTool).Pointer()
	for _, name := range toolNamesAnsweredBy(capabilityprotocol.AnsweredByRecord) {
		route, hasRoute := capabilityToolRouteFor(name)
		if !hasRoute {
			t.Errorf("%s is answered by the record and nothing routes it", name)
			continue
		}
		if reflect.ValueOf(route.Handler).Pointer() != carrier {
			t.Errorf("%s is answered by the record and is routed somewhere else", name)
		}
	}
}

func TestNothingElseIsCarriedToTheRecord(t *testing.T) {
	carrier := reflect.ValueOf(Service.invokeRecordTool).Pointer()
	for _, route := range capabilityToolRoutes {
		if reflect.ValueOf(route.Handler).Pointer() == carrier {
			t.Errorf("%s is carried to the record by a hand-written route rather than by its descriptor", route.ToolName)
		}
	}
}

func TestTheRouteTableNamesNoToolTheRecordAnswers(t *testing.T) {
	for _, route := range capabilityToolRoutes {
		if theRecordAnswers(route.ToolName) {
			t.Errorf("%s is answered by the record and the route table names it too", route.ToolName)
		}
	}
}
