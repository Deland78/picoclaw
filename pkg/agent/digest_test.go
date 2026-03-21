package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDigest_Success(t *testing.T) {
	resp := digestResponse{
		Posts: []linkedInPost{
			{
				Author:  "John Smith",
				Summary: "Building AI agents that actually work in production",
				PostURL: "https://linkedin.com/post/abc123",
			},
			{
				Author:  "Jane Doe",
				Summary: "The future of developer tools is conversational",
				PostURL: "https://linkedin.com/post/def456",
			},
		},
		ScrapedCount: 20,
		RankedCount:  2,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linkedin/digest", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := linkedInWorkerURL
	linkedInWorkerURL = func() string { return srv.URL }
	defer func() { linkedInWorkerURL = old }()

	al := &AgentLoop{}
	result, err := al.handleDigest(context.Background())
	require.NoError(t, err)

	assert.Contains(t, result, "LinkedIn Digest")
	assert.Contains(t, result, "2 posts ranked (scraped 20)")
	assert.Contains(t, result, "John Smith")
	assert.Contains(t, result, "Jane Doe")
	assert.Contains(t, result, "https://linkedin.com/post/abc123")
	assert.Contains(t, result, "20 posts reviewed, 2 selected")
}

func TestHandleDigest_EmptyPosts(t *testing.T) {
	resp := digestResponse{
		Posts:        []linkedInPost{},
		ScrapedCount: 10,
		RankedCount:  0,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := linkedInWorkerURL
	linkedInWorkerURL = func() string { return srv.URL }
	defer func() { linkedInWorkerURL = old }()

	al := &AgentLoop{}
	result, err := al.handleDigest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "No LinkedIn posts found.", result)
}

func TestHandleDigest_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	old := linkedInWorkerURL
	linkedInWorkerURL = func() string { return srv.URL }
	defer func() { linkedInWorkerURL = old }()

	al := &AgentLoop{}
	_, err := al.handleDigest(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestHandleDigest_ConnectionError(t *testing.T) {
	old := linkedInWorkerURL
	linkedInWorkerURL = func() string { return "http://localhost:1" }
	defer func() { linkedInWorkerURL = old }()

	al := &AgentLoop{}
	_, err := al.handleDigest(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "linkedin worker call failed")
}

func TestHandleDigest_TruncatesLongSummary(t *testing.T) {
	longSummary := ""
	for i := 0; i < 150; i++ {
		longSummary += "a"
	}

	resp := digestResponse{
		Posts: []linkedInPost{
			{Author: "Test Author", Summary: longSummary, PostURL: "https://example.com"},
		},
		ScrapedCount: 1,
		RankedCount:  1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := linkedInWorkerURL
	linkedInWorkerURL = func() string { return srv.URL }
	defer func() { linkedInWorkerURL = old }()

	al := &AgentLoop{}
	result, err := al.handleDigest(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result, "...")
	// Summary should be truncated to 120 chars (117 + "...")
	assert.NotContains(t, result, longSummary)
}
