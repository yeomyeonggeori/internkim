package centralplane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryWriterOfTheDeviceMirrorSpellsItTheSameWay(t *testing.T) {
	for _, path := range []string{
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
