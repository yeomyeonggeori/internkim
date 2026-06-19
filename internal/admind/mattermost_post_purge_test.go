package admind

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSoftDeletedMattermostPostPurgeCutoff(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	cutoff := softDeletedMattermostPostPurgeCutoff(now)
	expected := strconv.FormatInt(now.Add(-softDeletedMattermostPostGrace).UnixMilli(), 10)
	if cutoff != expected {
		t.Fatalf("cutoff %q does not match expected %q", cutoff, expected)
	}
}

func TestSoftDeletedMattermostPostPurgeQuery(t *testing.T) {
	query := softDeletedMattermostPostPurgeQuery("1750334400000")
	for _, fragment := range []string{
		"DELETE FROM reactions WHERE postid IN (SELECT id FROM posts WHERE deleteat > 0 AND deleteat < 1750334400000)",
		"DELETE FROM threadmemberships WHERE postid IN (",
		"DELETE FROM threads WHERE postid IN (",
		"DELETE FROM posts WHERE deleteat > 0 AND deleteat < 1750334400000",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q\nfull: %s", fragment, query)
		}
	}
	if strings.Index(query, "DELETE FROM posts WHERE") < strings.Index(query, "DELETE FROM reactions") {
		t.Fatalf("posts must be deleted after child rows: %s", query)
	}
}
