package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sipeed/picoclaw/pkg/skills"
)

func TestInstallSkillToolName(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())
	assert.Equal(t, "install_skill", tool.Name())
}

func TestInstallSkillToolMissingSlug(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())
	result := tool.Execute(context.Background(), map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "identifier is required and must be a non-empty string")
}

func TestInstallSkillToolEmptySlug(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())
	result := tool.Execute(context.Background(), map[string]any{
		"slug": "   ",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "identifier is required and must be a non-empty string")
}

func TestInstallSkillToolUnsafeSlug(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())

	cases := []string{
		"../etc/passwd",
		"path/traversal",
		"path\\traversal",
	}

	for _, slug := range cases {
		result := tool.Execute(context.Background(), map[string]any{
			"slug": slug,
		})
		assert.True(t, result.IsError, "slug %q should be rejected", slug)
		assert.Contains(t, result.ForLLM, "invalid slug")
	}
}

func TestInstallSkillToolAlreadyExists(t *testing.T) {
	workspace := t.TempDir()
	skillDir := filepath.Join(workspace, "skills", "existing-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	tool := NewInstallSkillTool(skills.NewRegistryManager(), workspace)
	result := tool.Execute(context.Background(), map[string]any{
		"slug":      "existing-skill",
		"registry":  "clawhub",
		"confirmed": true,
	})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "already installed")
}

func TestInstallSkillToolRegistryNotFound(t *testing.T) {
	workspace := t.TempDir()
	tool := NewInstallSkillTool(skills.NewRegistryManager(), workspace)
	result := tool.Execute(context.Background(), map[string]any{
		"slug":     "some-skill",
		"registry": "nonexistent",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "registry")
	assert.Contains(t, result.ForLLM, "not found")
}

func TestInstallSkillToolParameters(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())
	params := tool.Parameters()

	props, ok := params["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, props, "slug")
	assert.Contains(t, props, "version")
	assert.Contains(t, props, "registry")
	assert.Contains(t, props, "force")
	assert.Contains(t, props, "confirmed")

	required, ok := params["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "slug")
	assert.Contains(t, required, "registry")
}

func TestInstallSkillToolMissingRegistry(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())
	result := tool.Execute(context.Background(), map[string]any{
		"slug": "some-skill",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "invalid registry")
}

// --- mock skill registry for preview tests ---

type mockSkillRegistry struct {
	name          string
	meta          *skills.SkillMeta
	metaErr       error
	installResult *skills.InstallResult
	installErr    error
}

func (m *mockSkillRegistry) Name() string { return m.name }
func (m *mockSkillRegistry) Search(_ context.Context, _ string, _ int) ([]skills.SearchResult, error) {
	return nil, nil
}
func (m *mockSkillRegistry) GetSkillMeta(_ context.Context, _ string) (*skills.SkillMeta, error) {
	return m.meta, m.metaErr
}
func (m *mockSkillRegistry) DownloadAndInstall(_ context.Context, _, _, _ string) (*skills.InstallResult, error) {
	return m.installResult, m.installErr
}

func TestInstallSkillPreview_ReturnsPreviewWithoutConfirmed(t *testing.T) {
	mgr := skills.NewRegistryManager()
	mgr.AddRegistry(&mockSkillRegistry{
		name: "clawhub",
		meta: &skills.SkillMeta{
			Slug:          "linkedin-digest",
			DisplayName:   "LinkedIn Digest",
			Summary:       "Scrape and rank LinkedIn posts",
			LatestVersion: "1.2.0",
		},
	})

	workspace := t.TempDir()
	tool := NewInstallSkillTool(mgr, workspace)
	result := tool.Execute(context.Background(), map[string]any{
		"slug":     "linkedin-digest",
		"registry": "clawhub",
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "Preview: would install")
	assert.Contains(t, result.ForLLM, "LinkedIn Digest")
	assert.Contains(t, result.ForLLM, "clawhub registry")
	assert.Contains(t, result.ForLLM, "1.2.0")
	assert.Contains(t, result.ForLLM, "Scrape and rank LinkedIn posts")
	assert.Contains(t, result.ForLLM, "confirmed=true")

	// Verify no directory was created
	_, err := os.Stat(filepath.Join(workspace, "skills", "linkedin-digest"))
	assert.True(t, os.IsNotExist(err), "skill directory should not exist in preview mode")
}

func TestInstallSkillPreview_RegistryNotFound(t *testing.T) {
	tool := NewInstallSkillTool(skills.NewRegistryManager(), t.TempDir())
	result := tool.Execute(context.Background(), map[string]any{
		"slug":     "some-skill",
		"registry": "nonexistent",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "registry")
	assert.Contains(t, result.ForLLM, "not found")
}

func TestInstallSkillPreview_MetadataError(t *testing.T) {
	mgr := skills.NewRegistryManager()
	mgr.AddRegistry(&mockSkillRegistry{
		name:    "clawhub",
		metaErr: fmt.Errorf("network timeout"),
	})

	tool := NewInstallSkillTool(mgr, t.TempDir())
	result := tool.Execute(context.Background(), map[string]any{
		"slug":     "some-skill",
		"registry": "clawhub",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "failed to fetch skill metadata")
}
