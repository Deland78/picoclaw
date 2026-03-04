package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// setTestHome overrides the home directory for both Unix and Windows.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}
}

func TestLoadClaudeCodeCredentials_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	expiresAtMs := time.Now().Add(time.Hour).UnixMilli()
	credJSON := `{
		"claudeAiOauth": {
			"accessToken": "sk-ant-oat01-test-access-token",
			"refreshToken": "sk-ant-ort01-test-refresh-token",
			"expiresAt": ` + strconv.FormatInt(expiresAtMs, 10) + `
		}
	}`

	credDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte(credJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	cred, err := LoadClaudeCodeCredentials()
	if err != nil {
		t.Fatalf("LoadClaudeCodeCredentials() error: %v", err)
	}
	if cred == nil {
		t.Fatal("LoadClaudeCodeCredentials() returned nil")
	}
	if cred.AccessToken != "sk-ant-oat01-test-access-token" {
		t.Errorf("AccessToken = %q, want %q", cred.AccessToken, "sk-ant-oat01-test-access-token")
	}
	if cred.RefreshToken != "sk-ant-ort01-test-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", cred.RefreshToken, "sk-ant-ort01-test-refresh-token")
	}
	if cred.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", cred.Provider, "anthropic")
	}
	if cred.AuthMethod != "claude-code" {
		t.Errorf("AuthMethod = %q, want %q", cred.AuthMethod, "claude-code")
	}
	if cred.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
	expectedExpiry := time.UnixMilli(expiresAtMs)
	if diff := cred.ExpiresAt.Sub(expectedExpiry); diff < -time.Second || diff > time.Second {
		t.Errorf("ExpiresAt = %v, want ~%v", cred.ExpiresAt, expectedExpiry)
	}
}

func TestLoadClaudeCodeCredentials_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	cred, err := LoadClaudeCodeCredentials()
	if err != nil {
		t.Fatalf("LoadClaudeCodeCredentials() error: %v, want nil", err)
	}
	if cred != nil {
		t.Errorf("LoadClaudeCodeCredentials() = %+v, want nil", cred)
	}
}

func TestLoadClaudeCodeCredentials_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	credDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte("{corrupt"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	cred, err := LoadClaudeCodeCredentials()
	if err == nil {
		t.Fatal("LoadClaudeCodeCredentials() error = nil, want error")
	}
	if cred != nil {
		t.Errorf("LoadClaudeCodeCredentials() = %+v, want nil", cred)
	}
}

func TestLoadClaudeCodeCredentials_MissingToken(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	credJSON := `{
		"claudeAiOauth": {
			"accessToken": "",
			"refreshToken": "some-refresh",
			"expiresAt": 9999999999999
		}
	}`

	credDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte(credJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	cred, err := LoadClaudeCodeCredentials()
	if err == nil {
		t.Fatal("LoadClaudeCodeCredentials() error = nil, want error")
	}
	if cred != nil {
		t.Errorf("LoadClaudeCodeCredentials() = %+v, want nil", cred)
	}
}

func TestLoadClaudeCodeCredentials_ExpiredToken(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	expiresAtMs := time.Now().Add(-time.Hour).UnixMilli()
	credJSON := `{
		"claudeAiOauth": {
			"accessToken": "sk-ant-oat01-expired-token",
			"refreshToken": "sk-ant-ort01-refresh",
			"expiresAt": ` + strconv.FormatInt(expiresAtMs, 10) + `
		}
	}`

	credDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte(credJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Expired tokens should still be returned (caller decides what to do)
	cred, err := LoadClaudeCodeCredentials()
	if err != nil {
		t.Fatalf("LoadClaudeCodeCredentials() error: %v", err)
	}
	if cred == nil {
		t.Fatal("LoadClaudeCodeCredentials() returned nil, want expired credential")
	}
	if cred.AccessToken != "sk-ant-oat01-expired-token" {
		t.Errorf("AccessToken = %q, want %q", cred.AccessToken, "sk-ant-oat01-expired-token")
	}
	if !cred.IsExpired() {
		t.Error("IsExpired() = false, want true")
	}
}

func TestClaudeCodeCredentialPath(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	path := ClaudeCodeCredentialPath()
	want := filepath.Join(tmpDir, ".claude", ".credentials.json")
	if path != want {
		t.Errorf("ClaudeCodeCredentialPath() = %q, want %q", path, want)
	}
}
