package context

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveActor(t *testing.T) {
	// Save original environment
	origStashActor := os.Getenv("STASH_ACTOR")
	origUser := os.Getenv("USER")
	defer func() {
		os.Setenv("STASH_ACTOR", origStashActor)
		os.Setenv("USER", origUser)
	}()

	t.Run("priority 1: flag value takes precedence", func(t *testing.T) {
		os.Setenv("STASH_ACTOR", "env-actor")
		os.Setenv("USER", "env-user")

		result := ResolveActor("flag-actor")
		assert.Equal(t, "flag-actor", result)
	})

	t.Run("priority 2: STASH_ACTOR when no flag", func(t *testing.T) {
		os.Setenv("STASH_ACTOR", "env-actor")
		os.Setenv("USER", "env-user")

		result := ResolveActor("")
		assert.Equal(t, "env-actor", result)
	})

	t.Run("priority 3: USER when no flag or STASH_ACTOR", func(t *testing.T) {
		os.Unsetenv("STASH_ACTOR")
		os.Setenv("USER", "env-user")

		result := ResolveActor("")
		assert.Equal(t, "env-user", result)
	})

	t.Run("priority 4: unknown as fallback", func(t *testing.T) {
		os.Unsetenv("STASH_ACTOR")
		os.Unsetenv("USER")

		result := ResolveActor("")
		assert.Equal(t, "unknown", result)
	})

	t.Run("flag overrides both env vars", func(t *testing.T) {
		os.Setenv("STASH_ACTOR", "stash-actor")
		os.Setenv("USER", "system-user")

		result := ResolveActor("cli-actor")
		assert.Equal(t, "cli-actor", result)
	})

	t.Run("empty STASH_ACTOR falls through to USER", func(t *testing.T) {
		os.Setenv("STASH_ACTOR", "")
		os.Setenv("USER", "fallback-user")

		result := ResolveActor("")
		assert.Equal(t, "fallback-user", result)
	})

	t.Run("empty USER falls through to unknown", func(t *testing.T) {
		os.Unsetenv("STASH_ACTOR")
		os.Setenv("USER", "")

		result := ResolveActor("")
		assert.Equal(t, "unknown", result)
	})
}
