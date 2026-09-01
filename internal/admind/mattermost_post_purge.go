package admind

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

const softDeletedMattermostPostGrace = 10 * time.Minute

func (service *Service) purgeSoftDeletedMattermostPosts(ctx context.Context, now time.Time) error {
	cutoff := softDeletedMattermostPostPurgeCutoff(now)
	purgedCount, errorValue := service.softDeletedMattermostPostCount(ctx, cutoff)
	if errorValue != nil {
		return errorValue
	}
	if purgedCount == 0 {
		return nil
	}
	if _, errorValue := service.runPostgresMattermostQuery(ctx, softDeletedMattermostPostPurgeQuery(cutoff)); errorValue != nil {
		return errorValue
	}
	log.Printf("soft-deleted Mattermost post purge removed %d posts", purgedCount)
	return nil
}

func (service *Service) softDeletedMattermostPostCount(ctx context.Context, cutoff string) (int, error) {
	output, errorValue := service.runPostgresMattermostQuery(ctx, "SELECT count(*) FROM posts WHERE deleteat > 0 AND deleteat < "+cutoff)
	if errorValue != nil {
		return 0, errorValue
	}
	trimmedOutput := strings.TrimSpace(string(output))
	count, errorValue := strconv.Atoi(trimmedOutput)
	if errorValue != nil {
		return 0, fmt.Errorf("unexpected soft-deleted post count output %q: %w", trimmedOutput, errorValue)
	}
	return count, nil
}

func softDeletedMattermostPostPurgeCutoff(now time.Time) string {
	return strconv.FormatInt(now.Add(-softDeletedMattermostPostGrace).UnixMilli(), 10)
}

func softDeletedMattermostPostPurgeQuery(cutoff string) string {
	selection := "SELECT id FROM posts WHERE deleteat > 0 AND deleteat < " + cutoff
	statements := []string{
		"DELETE FROM reactions WHERE postid IN (" + selection + ")",
		"DELETE FROM threadmemberships WHERE postid IN (" + selection + ")",
		"DELETE FROM threads WHERE postid IN (" + selection + ")",
		"DELETE FROM posts WHERE deleteat > 0 AND deleteat < " + cutoff,
	}
	return strings.Join(statements, "; ")
}
