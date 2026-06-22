package hooks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostCommitRegeneratesRegistry(t *testing.T) {
	indexIdx := strings.Index(postCommit, "stardust index")
	require.GreaterOrEqual(t, indexIdx, 0, "post-commit hook must still index changed notes")

	registryIdx := strings.Index(postCommit, "stardust registry")
	require.GreaterOrEqual(t, registryIdx, 0, "post-commit hook must regenerate the docs registry")
	require.Greater(t, registryIdx, indexIdx, "registry line must come after the index line")

	// The registry line stays best-effort so it never fails the commit.
	registryLine := postCommit[registryIdx:]
	if nl := strings.IndexByte(registryLine, '\n'); nl >= 0 {
		registryLine = registryLine[:nl]
	}
	require.Contains(t, registryLine, "|| true", "registry line must not fail the commit")
}
