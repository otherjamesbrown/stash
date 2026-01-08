package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStash_Version(t *testing.T) {
	result := RunStash(t, "version")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "stash version")
}

func TestRunStash_Help(t *testing.T) {
	result := RunStash(t, "--help")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "Usage:")
	assert.Contains(t, result.Stdout, "stash")
}

func TestRunStash_InvalidCommand(t *testing.T) {
	result := RunStash(t, "nonexistent-command")

	assert.NotEqual(t, 0, result.ExitCode)
}

func TestRunStashInDir(t *testing.T) {
	tmpDir := TempDir(t)

	result := RunStashInDir(t, tmpDir, "version")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "stash version")
}

func TestMustSucceed_Success(t *testing.T) {
	result := MustSucceed(t, "version")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "stash version")
}

func TestMustFail_Failure(t *testing.T) {
	result := MustFail(t, "nonexistent-command")

	assert.NotEqual(t, 0, result.ExitCode)
}

func TestStashBinary(t *testing.T) {
	binary := stashBinary()
	require.NotEmpty(t, binary)
}
