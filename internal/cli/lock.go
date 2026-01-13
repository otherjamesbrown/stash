// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

// Lock represents a record lock for multi-agent coordination
type Lock struct {
	RecordID  string    `json:"record_id"`
	Agent     string    `json:"agent"`
	LockedAt  time.Time `json:"locked_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Stash     string    `json:"stash"`
}

// IsExpired returns true if the lock has expired
func (l *Lock) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// Error codes for lock operations
const (
	ErrCodeRecordLocked  = "RECORD_LOCKED"
	ErrCodeLockNotFound  = "LOCK_NOT_FOUND"
	ErrCodeLockExpired   = "LOCK_EXPIRED"
)

// Default lock timeout in seconds
const DefaultLockTimeout = 300

var (
	lockAgent   string
	lockTimeout int
)

var lockCmd = &cobra.Command{
	Use:   "lock <id>",
	Short: "Lock a record for exclusive access",
	Long: `Lock a record to prevent concurrent updates by other agents.

This command acquires an exclusive lock on a record, preventing other agents
from updating it until the lock is released or expires.

The lock is associated with an agent name (defaults to current actor).
Locks auto-expire after a timeout (default 300 seconds / 5 minutes).

Examples:
  stash lock inv-ex4j                           # Lock with default timeout
  stash lock inv-ex4j --agent worker-1          # Lock as specific agent
  stash lock inv-ex4j --timeout 600             # Lock for 10 minutes
  stash lock inv-ex4j --json                    # JSON output for parsing

AI Agent Examples:
  # Lock before processing
  LOCK=$(stash lock "$RECORD_ID" --agent "$AGENT_NAME" --json)
  if [ $? -eq 0 ]; then
      # Process record...
      stash set "$RECORD_ID" status="complete"
      stash unlock "$RECORD_ID"
  fi

Exit Codes:
  0  Success - lock acquired
  1  Record not found
  5  Record already locked by another agent`,
	Args: cobra.ExactArgs(1),
	RunE: runLock,
}

var unlockCmd = &cobra.Command{
	Use:   "unlock <id>",
	Short: "Unlock a record",
	Long: `Release a lock on a record.

This command releases an exclusive lock, allowing other agents to update
the record. The lock can be released by any agent (not just the owner).

Examples:
  stash unlock inv-ex4j
  stash unlock inv-ex4j --json

Exit Codes:
  0  Success - lock released
  1  Record not found (or no lock exists)`,
	Args: cobra.ExactArgs(1),
	RunE: runUnlock,
}

var locksCmd = &cobra.Command{
	Use:   "locks",
	Short: "List current locks",
	Long: `List all active (non-expired) locks in the stash.

Shows which records are locked, by which agent, and when the lock expires.

Examples:
  stash locks
  stash locks --json

Exit Codes:
  0  Success`,
	Args: cobra.NoArgs,
	RunE: runLocks,
}

func init() {
	lockCmd.Flags().StringVar(&lockAgent, "agent", "", "Agent name for the lock (default: current actor)")
	lockCmd.Flags().IntVar(&lockTimeout, "timeout", DefaultLockTimeout, "Lock timeout in seconds (default 300)")
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(unlockCmd)
	rootCmd.AddCommand(locksCmd)
}

func runLock(cmd *cobra.Command, args []string) error {
	recordID := args[0]

	// Resolve context
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			ExitNoStashDir()
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			ExitValidationError("no stash specified and multiple stashes exist (use --stash)", nil)
			return nil
		}
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Verify stash exists
	_, err = store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			ExitStashNotFound(ctx.Stash)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Verify record exists
	_, err = store.GetRecord(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			ExitRecordNotFound(recordID)
			return nil
		}
		if errors.Is(err, model.ErrRecordDeleted) {
			ExitRecordDeleted(recordID)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	// Determine agent name
	agent := lockAgent
	if agent == "" {
		agent = ctx.Actor
	}

	// Check for existing lock
	locks, err := loadLocks(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load locks: %w", err)
	}

	// Clean up expired locks while checking
	locks = cleanExpiredLocks(locks)

	// Check if record is already locked
	for _, lock := range locks {
		if lock.Stash == ctx.Stash && lock.RecordID == recordID {
			if lock.Agent == agent {
				// Same agent - refresh the lock
				lock.LockedAt = time.Now()
				lock.ExpiresAt = time.Now().Add(time.Duration(lockTimeout) * time.Second)
				if err := saveLocks(ctx.StashDir, locks); err != nil {
					return fmt.Errorf("failed to save locks: %w", err)
				}
				outputLock(lock)
				return nil
			}
			// Different agent - lock conflict
			ExitWithError(5, ErrCodeRecordLocked,
				fmt.Sprintf("record '%s' is locked by agent '%s' (expires %s)",
					recordID, lock.Agent, lock.ExpiresAt.Format(time.RFC3339)),
				map[string]interface{}{
					"record_id":  recordID,
					"locked_by":  lock.Agent,
					"locked_at":  lock.LockedAt,
					"expires_at": lock.ExpiresAt,
				})
			return nil
		}
	}

	// Create new lock
	now := time.Now()
	lock := &Lock{
		RecordID:  recordID,
		Agent:     agent,
		LockedAt:  now,
		ExpiresAt: now.Add(time.Duration(lockTimeout) * time.Second),
		Stash:     ctx.Stash,
	}
	locks = append(locks, lock)

	// Save locks
	if err := saveLocks(ctx.StashDir, locks); err != nil {
		return fmt.Errorf("failed to save locks: %w", err)
	}

	outputLock(lock)
	return nil
}

func runUnlock(cmd *cobra.Command, args []string) error {
	recordID := args[0]

	// Resolve context
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			ExitNoStashDir()
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			ExitValidationError("no stash specified and multiple stashes exist (use --stash)", nil)
			return nil
		}
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Load locks
	locks, err := loadLocks(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load locks: %w", err)
	}

	// Find and remove the lock
	found := false
	var newLocks []*Lock
	for _, lock := range locks {
		if lock.Stash == ctx.Stash && lock.RecordID == recordID {
			found = true
			continue // Remove this lock
		}
		newLocks = append(newLocks, lock)
	}

	if !found {
		ExitWithError(1, ErrCodeLockNotFound,
			fmt.Sprintf("no lock found for record '%s'", recordID),
			map[string]interface{}{"record_id": recordID})
		return nil
	}

	// Save updated locks
	if err := saveLocks(ctx.StashDir, newLocks); err != nil {
		return fmt.Errorf("failed to save locks: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"unlocked":  true,
			"record_id": recordID,
		}
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Unlocked %s\n", recordID)
	}

	return nil
}

func runLocks(cmd *cobra.Command, args []string) error {
	// Resolve context
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			ExitNoStashDir()
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			ExitValidationError("no stash specified and multiple stashes exist (use --stash)", nil)
			return nil
		}
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Load locks
	locks, err := loadLocks(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load locks: %w", err)
	}

	// Clean up expired locks
	locks = cleanExpiredLocks(locks)
	if err := saveLocks(ctx.StashDir, locks); err != nil {
		return fmt.Errorf("failed to save locks: %w", err)
	}

	// Filter to current stash
	var stashLocks []*Lock
	for _, lock := range locks {
		if lock.Stash == ctx.Stash {
			stashLocks = append(stashLocks, lock)
		}
	}

	// Output result
	if GetJSONOutput() {
		data, err := json.Marshal(stashLocks)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(stashLocks) == 0 {
			fmt.Println("No active locks")
		} else {
			for _, lock := range stashLocks {
				remaining := time.Until(lock.ExpiresAt).Round(time.Second)
				fmt.Printf("%s  locked by %s  expires in %s\n",
					lock.RecordID, lock.Agent, remaining)
			}
		}
	}

	return nil
}

// outputLock outputs lock information in the appropriate format
func outputLock(lock *Lock) {
	if GetJSONOutput() {
		data, _ := json.Marshal(lock)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Locked %s (expires %s)\n", lock.RecordID, lock.ExpiresAt.Format(time.RFC3339))
		if IsVerbose() {
			fmt.Printf("  agent: %s\n", lock.Agent)
			fmt.Printf("  locked_at: %s\n", lock.LockedAt.Format(time.RFC3339))
		}
	}
}

// locksFilePath returns the path to the locks file
func locksFilePath(stashDir string) string {
	return filepath.Join(stashDir, "locks.json")
}

// loadLocks loads all locks from the locks file
func loadLocks(stashDir string) ([]*Lock, error) {
	path := locksFilePath(stashDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Lock{}, nil
		}
		return nil, err
	}

	var locks []*Lock
	if err := json.Unmarshal(data, &locks); err != nil {
		return nil, err
	}
	return locks, nil
}

// saveLocks saves all locks to the locks file
func saveLocks(stashDir string, locks []*Lock) error {
	path := locksFilePath(stashDir)
	data, err := json.MarshalIndent(locks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// cleanExpiredLocks removes expired locks from the list
func cleanExpiredLocks(locks []*Lock) []*Lock {
	var active []*Lock
	for _, lock := range locks {
		if !lock.IsExpired() {
			active = append(active, lock)
		}
	}
	return active
}

// CheckLock checks if a record is locked by another agent.
// Returns the lock if found and not owned by the given agent, nil otherwise.
func CheckLock(stashDir, stashName, recordID, agent string) (*Lock, error) {
	locks, err := loadLocks(stashDir)
	if err != nil {
		return nil, err
	}

	for _, lock := range locks {
		if lock.Stash == stashName && lock.RecordID == recordID {
			// Skip expired locks
			if lock.IsExpired() {
				continue
			}
			// If locked by a different agent, return the lock
			if lock.Agent != agent {
				return lock, nil
			}
		}
	}
	return nil, nil
}

// ExitRecordLocked outputs an error when a record is locked by another agent
func ExitRecordLocked(recordID string, lock *Lock) {
	ExitWithError(5, ErrCodeRecordLocked,
		fmt.Sprintf("record '%s' is locked by agent '%s'", recordID, lock.Agent),
		map[string]interface{}{
			"record_id":  recordID,
			"locked_by":  lock.Agent,
			"locked_at":  lock.LockedAt,
			"expires_at": lock.ExpiresAt,
		})
}
