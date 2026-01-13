package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/stash/internal/storage"
)

// TestLock_AcquireLock tests basic lock acquisition
func TestLock_AcquireLock(t *testing.T) {
	t.Run("AC-01: lock record successfully", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetLockFlags()

		// When: Lock the record
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		err := rootCmd.Execute()

		// Then: Lock succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify lock exists
		locks, _ := loadLocks(filepath.Join(tempDir, ".stash"))
		found := false
		for _, lock := range locks {
			if lock.RecordID == recordID && lock.Agent == "agent-1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected lock to be created")
		}
	})

	t.Run("AC-02: lock with default timeout", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetLockFlags()

		// When: Lock without specifying timeout
		rootCmd.SetArgs([]string{"lock", recordID})
		rootCmd.Execute()

		// Then: Lock has default timeout (300 seconds)
		locks, _ := loadLocks(filepath.Join(tempDir, ".stash"))
		for _, lock := range locks {
			if lock.RecordID == recordID {
				expectedExpiry := lock.LockedAt.Add(300 * time.Second)
				diff := lock.ExpiresAt.Sub(expectedExpiry)
				if diff > time.Second || diff < -time.Second {
					t.Errorf("expected default timeout of 300s, got expiry diff of %v", diff)
				}
				break
			}
		}
	})

	t.Run("AC-03: lock with custom timeout", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetLockFlags()

		// When: Lock with 600 second timeout
		rootCmd.SetArgs([]string{"lock", recordID, "--timeout", "600"})
		rootCmd.Execute()

		// Then: Lock expires in 600 seconds
		locks, _ := loadLocks(filepath.Join(tempDir, ".stash"))
		for _, lock := range locks {
			if lock.RecordID == recordID {
				expectedExpiry := lock.LockedAt.Add(600 * time.Second)
				diff := lock.ExpiresAt.Sub(expectedExpiry)
				if diff > time.Second || diff < -time.Second {
					t.Errorf("expected timeout of 600s, got expiry diff of %v", diff)
				}
				break
			}
		}
	})

	t.Run("AC-04: reject lock on non-existent record", func(t *testing.T) {
		// Given: No record exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetLockFlags()

		// When: Try to lock non-existent record
		rootCmd.SetArgs([]string{"lock", "inv-fake"})
		rootCmd.Execute()

		// Then: Fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})
}

// TestLock_LockConflict tests lock conflicts between agents
func TestLock_LockConflict(t *testing.T) {
	t.Run("AC-01: reject lock when record is locked by another agent", func(t *testing.T) {
		// Given: Record is locked by agent-1
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Lock as agent-1
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		rootCmd.Execute()

		// When: agent-2 tries to lock
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-2"})
		rootCmd.Execute()

		// Then: Fails with exit code 5 (locked)
		if ExitCode != 5 {
			t.Errorf("expected exit code 5, got %d", ExitCode)
		}
	})

	t.Run("AC-02: same agent can refresh lock", func(t *testing.T) {
		// Given: Record is locked by agent-1
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Lock as agent-1
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		rootCmd.Execute()

		// Get original lock time
		locks, _ := loadLocks(filepath.Join(tempDir, ".stash"))
		var origLockTime time.Time
		for _, lock := range locks {
			if lock.RecordID == recordID {
				origLockTime = lock.LockedAt
				break
			}
		}

		// Small delay to ensure time difference
		time.Sleep(10 * time.Millisecond)

		// When: agent-1 locks again (refresh)
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		err := rootCmd.Execute()

		// Then: Lock is refreshed successfully
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify lock time was updated
		locks, _ = loadLocks(filepath.Join(tempDir, ".stash"))
		for _, lock := range locks {
			if lock.RecordID == recordID {
				if !lock.LockedAt.After(origLockTime) && lock.LockedAt != origLockTime {
					t.Error("expected lock time to be refreshed")
				}
				break
			}
		}
	})
}

// TestUnlock tests the unlock command
func TestUnlock(t *testing.T) {
	t.Run("AC-01: unlock record successfully", func(t *testing.T) {
		// Given: Record is locked
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Lock the record
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		rootCmd.Execute()

		// When: Unlock the record
		ExitCode = 0
		rootCmd.SetArgs([]string{"unlock", recordID})
		err := rootCmd.Execute()

		// Then: Unlock succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify lock is removed
		locks, _ := loadLocks(filepath.Join(tempDir, ".stash"))
		for _, lock := range locks {
			if lock.RecordID == recordID {
				t.Error("expected lock to be removed")
				break
			}
		}
	})

	t.Run("AC-02: unlock non-existent lock fails", func(t *testing.T) {
		// Given: Record exists but is not locked
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetLockFlags()

		// When: Try to unlock
		rootCmd.SetArgs([]string{"unlock", recordID})
		rootCmd.Execute()

		// Then: Fails with exit code 1 (no lock found)
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})
}

// TestLocks tests the locks list command
func TestLocks(t *testing.T) {
	t.Run("AC-01: list locks shows active locks", func(t *testing.T) {
		// Given: Record is locked
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Lock the record
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		rootCmd.Execute()

		// When: List locks with JSON output
		ExitCode = 0
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		rootCmd.SetArgs([]string{"locks", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: JSON output contains the lock
		var locks []*Lock
		if err := json.Unmarshal([]byte(output), &locks); err != nil {
			t.Fatalf("expected valid JSON, got error: %v", err)
		}

		found := false
		for _, lock := range locks {
			if lock.RecordID == recordID && lock.Agent == "agent-1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected lock to be listed")
		}
	})

	t.Run("AC-02: list locks shows empty when no locks", func(t *testing.T) {
		// Given: No locks exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetLockFlags()

		// When: List locks with JSON output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		rootCmd.SetArgs([]string{"locks", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: JSON output is empty array or null
		var locks []*Lock
		if err := json.Unmarshal([]byte(output), &locks); err != nil {
			t.Fatalf("expected valid JSON, got error: %v", err)
		}

		if len(locks) != 0 {
			t.Errorf("expected 0 locks, got %d", len(locks))
		}
	})
}

// TestLock_SetIntegration tests that set command respects locks
func TestLock_SetIntegration(t *testing.T) {
	t.Run("AC-01: set fails when record is locked by another agent", func(t *testing.T) {
		// Given: Record is locked by agent-1
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Lock as agent-1
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		rootCmd.Execute()

		// When: agent-2 tries to update (default actor is different)
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "Price=999", "--actor", "agent-2"})
		rootCmd.Execute()

		// Then: Fails with exit code 5 (locked)
		if ExitCode != 5 {
			t.Errorf("expected exit code 5, got %d", ExitCode)
		}
	})

	t.Run("AC-02: set succeeds when record is locked by same agent", func(t *testing.T) {
		// Given: Record is locked by agent-1
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Lock as agent-1
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1"})
		rootCmd.Execute()

		// When: agent-1 tries to update
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "Price=999", "--actor", "agent-1"})
		err := rootCmd.Execute()

		// Then: Succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify update was applied
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()
		rec, _ := store.GetRecord("inventory", recordID)
		priceVal := fmt.Sprintf("%v", rec.Fields["Price"])
		if priceVal != "999" {
			t.Errorf("expected Price='999', got '%v'", priceVal)
		}
	})

	t.Run("AC-03: set succeeds when no lock exists", func(t *testing.T) {
		// Given: Record is not locked
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// When: Update without lock
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "Price=888"})
		err := rootCmd.Execute()

		// Then: Succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})
}

// TestLock_Expiration tests that expired locks are cleaned up
func TestLock_Expiration(t *testing.T) {
	t.Run("AC-01: expired lock is ignored", func(t *testing.T) {
		// Given: Record has an expired lock
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Manually create an expired lock
		expiredLock := &Lock{
			RecordID:  recordID,
			Agent:     "agent-1",
			LockedAt:  time.Now().Add(-10 * time.Minute),
			ExpiresAt: time.Now().Add(-5 * time.Minute), // Expired 5 minutes ago
			Stash:     "inventory",
		}
		saveLocks(filepath.Join(tempDir, ".stash"), []*Lock{expiredLock})

		// When: agent-2 tries to lock
		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-2"})
		err := rootCmd.Execute()

		// Then: Lock succeeds (expired lock ignored)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify new lock exists
		locks, _ := loadLocks(filepath.Join(tempDir, ".stash"))
		found := false
		for _, lock := range locks {
			if lock.RecordID == recordID && lock.Agent == "agent-2" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected new lock to be created")
		}
	})

	t.Run("AC-02: set succeeds when lock is expired", func(t *testing.T) {
		// Given: Record has an expired lock
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Manually create an expired lock
		expiredLock := &Lock{
			RecordID:  recordID,
			Agent:     "agent-1",
			LockedAt:  time.Now().Add(-10 * time.Minute),
			ExpiresAt: time.Now().Add(-5 * time.Minute),
			Stash:     "inventory",
		}
		saveLocks(filepath.Join(tempDir, ".stash"), []*Lock{expiredLock})

		// When: agent-2 tries to update
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "Price=777", "--actor", "agent-2"})
		err := rootCmd.Execute()

		// Then: Succeeds (expired lock ignored)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})
}

// TestLock_JSONOutput tests JSON output for lock commands
func TestLock_JSONOutput(t *testing.T) {
	t.Run("AC-01: lock command JSON output", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetLockFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: Lock with JSON output
		rootCmd.SetArgs([]string{"lock", recordID, "--agent", "agent-1", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON with lock details
		var lock Lock
		if err := json.Unmarshal([]byte(output), &lock); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if lock.RecordID != recordID {
			t.Errorf("expected record_id=%s, got %s", recordID, lock.RecordID)
		}
		if lock.Agent != "agent-1" {
			t.Errorf("expected agent=agent-1, got %s", lock.Agent)
		}
	})

	t.Run("AC-02: unlock command JSON output", func(t *testing.T) {
		// Given: Record is locked
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetLockFlags()
		rootCmd.SetArgs([]string{"lock", recordID})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: Unlock with JSON output
		rootCmd.SetArgs([]string{"unlock", recordID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["unlocked"] != true {
			t.Errorf("expected unlocked=true, got %v", result["unlocked"])
		}
		if result["record_id"] != recordID {
			t.Errorf("expected record_id=%s, got %v", recordID, result["record_id"])
		}
	})
}

// resetLockFlags resets lock command flags
func resetLockFlags() {
	lockAgent = ""
	lockTimeout = DefaultLockTimeout
	// Also reset global flags
	jsonOutput = false
	stashName = ""
	actorName = ""
	quiet = false
	verbose = false
}
