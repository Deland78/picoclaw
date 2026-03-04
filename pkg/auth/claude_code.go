package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// claudeCodeCredFile holds the structure of ~/.claude/.credentials.json.
type claudeCodeCredFile struct {
	ClaudeAiOauth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"` // unix milliseconds
	} `json:"claudeAiOauth"`
}

// ClaudeCodeCredentialPath returns the path to Claude Code's credential file.
func ClaudeCodeCredentialPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".credentials.json")
}

// LoadClaudeCodeCredentials reads Claude Code CLI's OAuth tokens from
// ~/.claude/.credentials.json and returns them as an AuthCredential.
// Returns (nil, nil) if the file does not exist (not logged in).
func LoadClaudeCodeCredentials() (*AuthCredential, error) {
	path := ClaudeCodeCredentialPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading Claude Code credentials: %w", err)
	}

	var cf claudeCodeCredFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing Claude Code credentials: %w", err)
	}

	if cf.ClaudeAiOauth.AccessToken == "" {
		return nil, fmt.Errorf("Claude Code credentials file has no access token")
	}

	var expiresAt time.Time
	if cf.ClaudeAiOauth.ExpiresAt > 0 {
		expiresAt = time.UnixMilli(cf.ClaudeAiOauth.ExpiresAt)
	}

	return &AuthCredential{
		AccessToken:  cf.ClaudeAiOauth.AccessToken,
		RefreshToken: cf.ClaudeAiOauth.RefreshToken,
		ExpiresAt:    expiresAt,
		Provider:     "anthropic",
		AuthMethod:   "claude-code",
	}, nil
}
