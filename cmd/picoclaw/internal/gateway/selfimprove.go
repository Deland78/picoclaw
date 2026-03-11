package gateway

import (
	"fmt"

	"github.com/sipeed/picoclaw/pkg/cron"
)

// seedSelfImproveCron adds a daily self-improvement review cron job if one does not already exist.
func seedSelfImproveCron(cronService *cron.CronService) {
	// Check if a self-improvement job already exists
	jobs := cronService.ListJobs(true)
	for _, job := range jobs {
		if job.Name == "self-improvement-review" {
			return // Already seeded
		}
	}

	prompt := "Review .learnings/ directory for self-improvement: " +
		"1) Read .learnings/LEARNINGS.md, ERRORS.md, FEATURE_REQUESTS.md. " +
		"2) For pending entries with high/critical priority, evaluate promotion. " +
		"3) For entries with 3+ See Also links, promote the pattern to memory/MEMORY.md. " +
		"4) For entries older than 30 days with no recurrence, mark as wont_fix. " +
		"5) Write a brief summary of actions taken."

	// Add a daily review job at 03:00
	_, err := cronService.AddJob(
		"self-improvement-review",
		cron.CronSchedule{Kind: "cron", Expr: "0 3 * * *"},
		prompt,
		false, // deliver=false: process through agent, don't deliver to user
		"", "",
	)
	if err != nil {
		fmt.Printf("Warning: failed to seed self-improvement cron job: %v\n", err)
	} else {
		fmt.Println("\u2713 Self-improvement review cron job seeded (daily at 03:00)")
	}
}
