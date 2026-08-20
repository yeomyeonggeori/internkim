package admind

import (
	"strings"
	"testing"
)

func TestTwoCopiesThatAgreeReportNothingToLookAt(t *testing.T) {
	report := describeFlowTaskComparison(flowTaskComparison{Compared: 610, Identical: 610, DifferingBy: map[string]int{}})

	if report != "610 compared, 610 identical" {
		t.Fatalf("a clean comparison is the one line that lets somebody flip the switch: %q", report)
	}
}

func TestTheReportLeadsWithTheFieldThatDiffersMost(t *testing.T) {
	report := describeFlowTaskComparison(flowTaskComparison{
		Compared:    610,
		Identical:   64,
		DifferingBy: map[string]int{"size": 3, "startDate": 539, "status": 4},
		DeviceOnly:  9,
	})

	lines := strings.Split(report, "\n")
	if !strings.Contains(lines[1], "startDate differs on 539") {
		t.Fatalf("539 is the thing to look at, not 3: %q", lines[1])
	}
	if !strings.Contains(report, "flow-central-backfill") {
		t.Fatal("a device-only count is actionable, so the report says what the action is")
	}
}

func TestAFieldTheCentralPlaneDoesNotCarryIsNotADifference(t *testing.T) {
	deviceTask := flowTask{Content: "업무", Status: "진행", StatusRank: 1024, MattermostPostID: "post-1", Flag: 1}
	centralTask := flowTask{Content: "업무", Status: "진행"}

	if differences := flowTaskDifferences(deviceTask, centralTask); len(differences) != 0 {
		t.Fatalf("statusRank, flag and the post id are the device's own bookkeeping: %v", differences)
	}
}

func TestEveryFieldTheCentralPlaneCarriesIsCompared(t *testing.T) {
	deviceTask := flowTask{
		Content: "a", Status: "진행", StartDate: "2026-08-20", EndDate: "2026-08-21",
		Business: "b", Type: "c", Size: "M", ParticipantIDs: []string{"one"},
	}
	centralTask := flowTask{
		Content: "z", Status: "완료", StartDate: "2026-08-19", EndDate: "2026-08-22",
		Business: "y", Type: "x", Size: "L", ParticipantIDs: []string{"two"},
	}

	if differences := flowTaskDifferences(deviceTask, centralTask); len(differences) != 8 {
		t.Fatalf("a field left out of the comparison is a field the switch moves silently: %v", differences)
	}
}
