package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReauthGmailTool_Metadata(t *testing.T) {
	tool := NewReauthGmailTool()

	if tool.Name() != "reauth_gmail" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "reauth_gmail")
	}

	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}

	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("Parameters() type = %v, want 'object'", params["type"])
	}

	// Should have no required parameters
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Parameters() should have properties map")
	}
	if len(props) != 0 {
		t.Errorf("Parameters() should have 0 properties, got %d", len(props))
	}
}

func TestReauthGmailTool_DeleteToken(t *testing.T) {
	// Create a temp directory simulating picoassist data structure
	tmpDir := t.TempDir()
	tokensDir := filepath.Join(tmpDir, "data", "tokens")
	if err := os.MkdirAll(tokensDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tokenPath := filepath.Join(tokensDir, "gmail_token.json")
	if err := os.WriteFile(tokenPath, []byte(`{"token": "stale"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatal("token file should exist before deletion")
	}

	// Call the internal delete helper
	err := deleteGmailToken(tmpDir)
	if err != nil {
		t.Fatalf("deleteGmailToken() returned error: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("token file should be deleted")
	}
}

func TestReauthGmailTool_DeleteToken_NotExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Should not error when token file doesn't exist
	err := deleteGmailToken(tmpDir)
	if err != nil {
		t.Fatalf("deleteGmailToken() should not error when file doesn't exist, got: %v", err)
	}
}
