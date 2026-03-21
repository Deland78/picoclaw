package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// linkedInWorkerURL returns the LinkedIn worker base URL from env or default.
var linkedInWorkerURL = func() string {
	if u := os.Getenv("LINKEDIN_WORKER_URL"); u != "" {
		return u
	}
	return "http://localhost:8003"
}

type linkedInPost struct {
	PostID          string  `json:"post_id"`
	Author          string  `json:"author"`
	Content         string  `json:"content"`
	PostURL         string  `json:"post_url"`
	FirstCommentURL string  `json:"first_comment_url"`
	Summary         string  `json:"summary"`
	RankScore       float64 `json:"rank_score"`
}

type digestResponse struct {
	Posts        []linkedInPost `json:"posts"`
	ScrapedCount int            `json:"scraped_count"`
	RankedCount  int            `json:"ranked_count"`
}

// handleDigest calls the LinkedIn worker API and returns a plain-text digest.
func (al *AgentLoop) handleDigest(ctx context.Context) (string, error) {
	workerURL := linkedInWorkerURL()
	body := bytes.NewReader([]byte(`{"max_posts":20}`))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, workerURL+"/linkedin/digest", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("linkedin worker call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("linkedin worker returned status %d", resp.StatusCode)
	}

	var result digestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode digest response: %w", err)
	}

	if len(result.Posts) == 0 {
		return "No LinkedIn posts found.", nil
	}

	// Format as plain text
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "LinkedIn Digest — %d posts ranked (scraped %d)\n\n",
		len(result.Posts), result.ScrapedCount)

	for i, p := range result.Posts {
		fmt.Fprintf(&buf, "%d. %s\n", i+1, p.Author)
		summary := p.Summary
		if summary == "" {
			summary = p.Content
		}
		if len(summary) > 120 {
			summary = summary[:117] + "..."
		}
		fmt.Fprintf(&buf, "   %s\n", summary)
		if p.PostURL != "" {
			fmt.Fprintf(&buf, "   %s\n", p.PostURL)
		}
		if i < len(result.Posts)-1 {
			fmt.Fprintln(&buf)
		}
	}

	fmt.Fprintf(&buf, "\n%d posts reviewed, %d selected", result.ScrapedCount, len(result.Posts))
	return buf.String(), nil
}
