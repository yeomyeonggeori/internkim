package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const learningOverviewPath = "/diagnostics/learning"

type learningOverview struct {
	Settings struct {
		Enabled     bool `json:"enabled"`
		ActiveLimit int  `json:"activeLimit"`
	} `json:"settings"`
	Skills           []learnedSkill   `json:"skills"`
	SoulHistory      []soulRevision   `json:"soulHistory"`
	Reviews          []learningReview `json:"reviews"`
	PendingCount     int              `json:"pendingCount"`
	ReviewedCount    int              `json:"reviewedCount"`
	LastReview       time.Time        `json:"lastReview"`
	LastObservedTask time.Time        `json:"lastObservedTask"`
}

type learnedSkill struct {
	ID          string    `json:"id"`
	Version     int       `json:"version"`
	Audience    string    `json:"audience"`
	Description string    `json:"description"`
	Instruction string    `json:"instruction"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type soulRevision struct {
	Version   int             `json:"version"`
	Document  json.RawMessage `json:"document"`
	Reason    string          `json:"reason"`
	CreatedAt time.Time       `json:"createdAt"`
	Origin    string          `json:"origin"`
}

type learningReview struct {
	ID       string `json:"id"`
	Audience string `json:"audience"`
	Decision struct {
		Action  string `json:"action"`
		SkillID string `json:"skillID"`
		Reason  string `json:"reason"`
	} `json:"decision"`
	Error string `json:"error"`
}

func runLearning() {
	if errorValue := runLearningArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runLearningArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printLearningUsage()
		return nil
	}
	client, errorValue := resolveAdminAPIClient(arguments)
	if errorValue != nil {
		return errorValue
	}
	return showLearningWithClient(arguments, client)
}

func showLearningWithClient(arguments []string, client adminAPIClient) error {
	if hasCommandArgument(arguments, "--json") {
		document, errorValue := client.request("GET", learningOverviewPath, nil, nil)
		if errorValue != nil {
			return errorValue
		}
		fmt.Fprintln(runCommandOutput, string(document))
		return nil
	}
	var overview learningOverview
	if _, errorValue := client.request("GET", learningOverviewPath, nil, &overview); errorValue != nil {
		return errorValue
	}
	printLearningOverview(overview)
	return nil
}

func printLearningOverview(overview learningOverview) {
	fmt.Fprintf(runCommandOutput, "enabled: %t  activeLimit: %d  pending: %d  reviewed: %d  lastReview: %s  lastObservedTask: %s\n",
		overview.Settings.Enabled, overview.Settings.ActiveLimit, overview.PendingCount, overview.ReviewedCount,
		formatLearningTime(overview.LastReview), formatLearningTime(overview.LastObservedTask))
	fmt.Fprintf(runCommandOutput, "\nreviews (%d):\n", len(overview.Reviews))
	for _, review := range overview.Reviews {
		fmt.Fprintf(runCommandOutput, "  %s  %-22s %-8s %-24s %s%s\n", review.ID, review.Audience, review.Decision.Action, review.Decision.SkillID, truncateTaskText(review.Decision.Reason, taskSummaryPreviewLimit), learningReviewError(review.Error))
	}
	fmt.Fprintf(runCommandOutput, "\nskills (%d):\n", len(overview.Skills))
	for _, skill := range overview.Skills {
		fmt.Fprintf(runCommandOutput, "  %-32s v%-3d %-8s %-22s %6dB  %s  %s\n", skill.ID, skill.Version, skill.Status, skill.Audience, len(skill.Instruction), formatLearningTime(skill.UpdatedAt), truncateTaskText(skill.Description, taskSummaryPreviewLimit))
	}
	fmt.Fprintf(runCommandOutput, "\nsoul revisions (%d):\n", len(overview.SoulHistory))
	for _, revision := range overview.SoulHistory {
		fmt.Fprintf(runCommandOutput, "  v%-3d %-10s %6dB  %s  %s\n", revision.Version, revision.Origin, len(revision.Document), formatLearningTime(revision.CreatedAt), truncateTaskText(revision.Reason, taskSummaryPreviewLimit))
	}
}

func learningReviewError(reviewError string) string {
	if reviewError == "" {
		return ""
	}
	return "  error: " + truncateTaskText(reviewError, taskSummaryPreviewLimit)
}

func formatLearningTime(moment time.Time) string {
	if moment.IsZero() {
		return "never"
	}
	return moment.Local().Format("2006-01-02 15:04")
}

func printLearningUsage() {
	fmt.Fprintln(runCommandOutput, "Usage: internkim learning [--json]")
	fmt.Fprintln(runCommandOutput, "Shows the learning reviewer's settings, every review it ran, the skills it keeps, and the soul revisions.")
	fmt.Fprintln(runCommandOutput, "Options:")
	fmt.Fprintln(runCommandOutput, "  --json  Print the raw overview, including full skill instructions and soul documents")
}
