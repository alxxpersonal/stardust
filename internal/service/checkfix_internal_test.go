package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/convention"
)

// TestFixableDocFieldsSchemaDerived asserts CheckFix derives fixability from the
// collection schema: a required field with a derivation (type, created, updated)
// is fixable; title and status (no derivation) and the optional ref fields are not.
func TestFixableDocFieldsSchemaDerived(t *testing.T) {
	fields, ok := convention.DocFields(t.TempDir(), "spec")
	require.True(t, ok)

	fixable := fixableDocFields(fields)
	require.True(t, fixable["type"])
	require.True(t, fixable["created"])
	require.True(t, fixable["updated"])
	require.False(t, fixable["title"])
	require.False(t, fixable["status"])
	require.False(t, fixable["governs"])
	require.False(t, fixable["related"])
}
