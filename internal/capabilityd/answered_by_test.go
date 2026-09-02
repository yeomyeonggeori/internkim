package capabilityd

import (
	"reflect"
	"sort"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

// Deleted by step 4 of internkim#1254, with the Go handlers it names.
var recordToolsStillAnsweredInGo = []string{
	"event_add",
	"event_delete",
	"event_list",
	"event_update",
	"person_list",
	"task_add",
	"task_delete",
	"task_list",
	"task_update",
}

func toolNamesAnsweredBy(answerer string) []string {
	names := []string{}
	for _, descriptor := range capabilities.RegisteredToolDescriptors() {
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
	for _, descriptor := range capabilities.RegisteredToolDescriptors() {
		if !answerers[descriptor.AnsweredBy] {
			t.Errorf("%s answeredBy is %q", descriptor.CanonicalName, descriptor.AnsweredBy)
		}
	}
}

func TestTheRecordsToolsAreCarriedToTheRecord(t *testing.T) {
	stillInGo := map[string]bool{}
	for _, name := range recordToolsStillAnsweredInGo {
		stillInGo[name] = true
	}
	carrier := reflect.ValueOf(Service.invokeRecordTool).Pointer()

	for _, name := range toolNamesAnsweredBy(capabilityprotocol.AnsweredByRecord) {
		route, hasRoute := capabilityToolRouteFor(name)
		if !hasRoute {
			t.Errorf("%s is answered by the record and nothing routes it", name)
			continue
		}
		carriesToTheRecord := reflect.ValueOf(route.Handler).Pointer() == carrier
		if carriesToTheRecord && stillInGo[name] {
			t.Errorf("%s carries to the record now; take it out of recordToolsStillAnsweredInGo", name)
		}
		if !carriesToTheRecord && !stillInGo[name] {
			t.Errorf("%s is answered by the record and is routed somewhere else", name)
		}
	}
}

func TestNothingElseIsCarriedToTheRecord(t *testing.T) {
	carrier := reflect.ValueOf(Service.invokeRecordTool).Pointer()
	for _, route := range capabilityToolRoutes {
		if reflect.ValueOf(route.Handler).Pointer() != carrier {
			continue
		}
		descriptor, hasDescriptor := capabilityToolDescriptorFor(route.ToolName)
		if !hasDescriptor {
			t.Errorf("%s is carried to the record and no descriptor offers it", route.ToolName)
			continue
		}
		if descriptor.AnsweredBy != capabilityprotocol.AnsweredByRecord {
			t.Errorf("%s is carried to the record and its descriptor says %s answers it", route.ToolName, descriptor.AnsweredBy)
		}
	}
}

func TestTheCarriersOwnListIsTheDescriptorsList(t *testing.T) {
	stillInGo := map[string]bool{}
	for _, name := range recordToolsStillAnsweredInGo {
		stillInGo[name] = true
	}
	expected := []string{}
	for _, name := range toolNamesAnsweredBy(capabilityprotocol.AnsweredByRecord) {
		if !stillInGo[name] {
			expected = append(expected, name)
		}
	}

	carried := []string{}
	for name := range toolsTheRecordRuns {
		carried = append(carried, name)
	}
	sort.Strings(carried)

	if !reflect.DeepEqual(carried, expected) {
		t.Errorf("the carrier runs %v; the descriptors say %v", carried, expected)
	}
}

func TestTheUnfinishedListNamesOnlyRecordTools(t *testing.T) {
	answeredByTheRecord := map[string]bool{}
	for _, name := range toolNamesAnsweredBy(capabilityprotocol.AnsweredByRecord) {
		answeredByTheRecord[name] = true
	}
	for _, name := range recordToolsStillAnsweredInGo {
		if !answeredByTheRecord[name] {
			t.Errorf("%s is listed as unfinished and no descriptor says the record answers it", name)
		}
	}
}
