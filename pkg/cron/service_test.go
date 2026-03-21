package cron

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveStore_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not enforced on Windows")
	}

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cron", "jobs.json")

	cs := NewCronService(storePath, nil)

	_, err := cs.AddJob("test", CronSchedule{Kind: "every", EveryMS: int64Ptr(60000)}, "hello", false, "cli", "direct")
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("cron store has permission %04o, want 0600", perm)
	}
}

func TestRecomputeNextRuns_PastDueAtJob_DeleteAfterRun(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cron", "jobs.json")

	cs := NewCronService(storePath, nil)

	pastTime := time.Now().Add(-1 * time.Hour).UnixMilli()
	job := CronJob{
		ID:      "past-due-delete",
		Name:    "past due job (delete)",
		Enabled: true,
		Schedule: CronSchedule{
			Kind: "at",
			AtMS: int64Ptr(pastTime),
		},
		State: CronJobState{
			NextRunAtMS: int64Ptr(pastTime),
		},
		DeleteAfterRun: true,
		CreatedAtMS:    pastTime,
		UpdatedAtMS:    pastTime,
	}
	cs.store.Jobs = append(cs.store.Jobs, job)
	cs.saveStoreUnsafe()

	// Simulate gateway restart: recomputeNextRuns should handle past-due job
	cs.recomputeNextRuns()

	// Job should be removed since it's past-due with deleteAfterRun=true
	for _, j := range cs.store.Jobs {
		if j.ID == "past-due-delete" {
			t.Errorf("past-due job with deleteAfterRun=true should have been removed, but still exists")
		}
	}
}

func TestRecomputeNextRuns_PastDueAtJob_NoDelete(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cron", "jobs.json")

	cs := NewCronService(storePath, nil)

	pastTime := time.Now().Add(-1 * time.Hour).UnixMilli()
	job := CronJob{
		ID:      "past-due-keep",
		Name:    "past due job (keep)",
		Enabled: true,
		Schedule: CronSchedule{
			Kind: "at",
			AtMS: int64Ptr(pastTime),
		},
		State: CronJobState{
			NextRunAtMS: int64Ptr(pastTime),
		},
		DeleteAfterRun: false,
		CreatedAtMS:    pastTime,
		UpdatedAtMS:    pastTime,
	}
	cs.store.Jobs = append(cs.store.Jobs, job)
	cs.saveStoreUnsafe()

	cs.recomputeNextRuns()

	// Job should still exist but be disabled
	var found *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == "past-due-keep" {
			found = &cs.store.Jobs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("past-due job with deleteAfterRun=false should still exist")
	}
	if found.Enabled {
		t.Error("past-due job should be disabled")
	}
	if found.State.NextRunAtMS != nil {
		t.Error("past-due job nextRunAtMS should be nil")
	}
}

func TestRecomputeNextRuns_FutureAtJob_Untouched(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cron", "jobs.json")

	cs := NewCronService(storePath, nil)

	futureTime := time.Now().Add(1 * time.Hour).UnixMilli()
	job := CronJob{
		ID:      "future-job",
		Name:    "future job",
		Enabled: true,
		Schedule: CronSchedule{
			Kind: "at",
			AtMS: int64Ptr(futureTime),
		},
		State: CronJobState{
			NextRunAtMS: int64Ptr(futureTime),
		},
		DeleteAfterRun: true,
		CreatedAtMS:    futureTime,
		UpdatedAtMS:    futureTime,
	}
	cs.store.Jobs = append(cs.store.Jobs, job)
	cs.saveStoreUnsafe()

	cs.recomputeNextRuns()

	var found *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == "future-job" {
			found = &cs.store.Jobs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("future job should still exist")
	}
	if !found.Enabled {
		t.Error("future job should still be enabled")
	}
	if found.State.NextRunAtMS == nil || *found.State.NextRunAtMS != futureTime {
		t.Error("future job nextRunAtMS should be unchanged")
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
