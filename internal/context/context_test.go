package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	// Save original environment
	origStashActor := os.Getenv("STASH_ACTOR")
	origUser := os.Getenv("USER")
	origStashDefault := os.Getenv("STASH_DEFAULT")
	defer func() {
		os.Setenv("STASH_ACTOR", origStashActor)
		os.Setenv("USER", origUser)
		os.Setenv("STASH_DEFAULT", origStashDefault)
	}()

	t.Run("resolves all context fields", func(t *testing.T) {
		os.Setenv("STASH_ACTOR", "test-actor")
		os.Unsetenv("STASH_DEFAULT")

		ctx, err := Resolve("", "test-stash")
		require.NoError(t, err)

		assert.Equal(t, "test-actor", ctx.Actor)
		assert.Equal(t, "test-stash", ctx.Stash)
		// Branch should be detected from current git repo
		assert.NotEmpty(t, ctx.Branch)
	})

	t.Run("uses flag values over environment", func(t *testing.T) {
		os.Setenv("STASH_ACTOR", "env-actor")
		os.Setenv("STASH_DEFAULT", "env-stash")

		ctx, err := Resolve("flag-actor", "flag-stash")
		require.NoError(t, err)

		assert.Equal(t, "flag-actor", ctx.Actor)
		assert.Equal(t, "flag-stash", ctx.Stash)
	})

	t.Run("auto-detects stash when flag empty", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		// Create temp directory with .stash containing one stash
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "auto-stash"), 0755))

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		ctx, err := Resolve("actor", "")
		require.NoError(t, err)

		assert.Equal(t, stashDir, ctx.StashDir)
		assert.Equal(t, "auto-stash", ctx.Stash)
	})

	t.Run("returns empty stash when multiple exist", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		// Create temp directory with .stash containing multiple stashes
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash1"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash2"), 0755))

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		ctx, err := Resolve("actor", "")
		require.NoError(t, err)

		assert.Equal(t, stashDir, ctx.StashDir)
		assert.Empty(t, ctx.Stash)
	})
}

func TestResolveRequired(t *testing.T) {
	// Save original environment
	origStashActor := os.Getenv("STASH_ACTOR")
	origStashDefault := os.Getenv("STASH_DEFAULT")
	defer func() {
		os.Setenv("STASH_ACTOR", origStashActor)
		os.Setenv("STASH_DEFAULT", origStashDefault)
	}()

	t.Run("returns error when no stash dir", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		// Create temp directory without .stash
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		_, err := ResolveRequired("actor", "")
		assert.ErrorIs(t, err, ErrNoStashDir)
	})

	t.Run("returns error when multiple stashes and no flag", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		// Create temp directory with multiple stashes
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash1"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash2"), 0755))

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		_, err := ResolveRequired("actor", "")
		assert.ErrorIs(t, err, ErrNoStash)
	})

	t.Run("succeeds with single stash", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		// Create temp directory with single stash
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "only"), 0755))

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		ctx, err := ResolveRequired("actor", "")
		require.NoError(t, err)
		assert.Equal(t, "only", ctx.Stash)
	})

	t.Run("succeeds with stash flag", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		// Create temp directory with multiple stashes
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash1"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash2"), 0755))

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		ctx, err := ResolveRequired("actor", "stash1")
		require.NoError(t, err)
		assert.Equal(t, "stash1", ctx.Stash)
	})
}

func TestContext_StashPath(t *testing.T) {
	t.Run("returns full path when both set", func(t *testing.T) {
		ctx := &Context{
			StashDir: "/path/to/.stash",
			Stash:    "my-stash",
		}

		result := ctx.StashPath()
		assert.Equal(t, "/path/to/.stash/my-stash", result)
	})

	t.Run("returns empty when StashDir empty", func(t *testing.T) {
		ctx := &Context{
			StashDir: "",
			Stash:    "my-stash",
		}

		result := ctx.StashPath()
		assert.Empty(t, result)
	})

	t.Run("returns empty when Stash empty", func(t *testing.T) {
		ctx := &Context{
			StashDir: "/path/to/.stash",
			Stash:    "",
		}

		result := ctx.StashPath()
		assert.Empty(t, result)
	})

	t.Run("returns empty when both empty", func(t *testing.T) {
		ctx := &Context{
			StashDir: "",
			Stash:    "",
		}

		result := ctx.StashPath()
		assert.Empty(t, result)
	})
}

func TestErrorMessages(t *testing.T) {
	t.Run("ErrNoStashDir has descriptive message", func(t *testing.T) {
		assert.Contains(t, ErrNoStashDir.Error(), ".stash")
	})

	t.Run("ErrNoStash has descriptive message", func(t *testing.T) {
		assert.Contains(t, ErrNoStash.Error(), "--stash")
	})
}
