package centralplane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Three languages write this one identifier and none of them can import the
// others: the daemon here, the relay that carries a device's calendar, and the
// trigger that reads it to tell imported history from an ask somebody made.
func TestEveryWriterOfTheDeviceMirrorSpellsItTheSameWay(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "host", "relay", "calendar-event-as-task.ts"),
		filepath.Join("..", "..", "supabase", "migrations",
			"20260904000012_a_carried_request_names_no_requester.sql"),
	} {
		source, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !strings.Contains(string(source), DeviceMirrorSource) {
			t.Fatalf("%s no longer names %s, so a carried task is a different row to it", path, DeviceMirrorSource)
		}
	}
}
