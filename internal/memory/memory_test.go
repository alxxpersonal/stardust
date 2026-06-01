package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateViewAppend(t *testing.T) {
	m := New(t.TempDir())
	require.NoError(t, m.Create("notes/a.md", "# A\nhello"))
	v, err := m.View("notes/a.md")
	require.NoError(t, err)
	require.Contains(t, v, "hello")

	require.NoError(t, m.Append("notes/a.md", "\nworld"))
	v, _ = m.View("notes/a.md")
	require.Contains(t, v, "world")

	require.Error(t, m.Create("notes/a.md", "x")) // already exists
}

func TestStrReplaceUniqueness(t *testing.T) {
	m := New(t.TempDir())
	require.NoError(t, m.Create("a.md", "foo bar foo"))
	require.Error(t, m.StrReplace("a.md", "foo", "X")) // not unique
	require.NoError(t, m.StrReplace("a.md", "bar", "BAZ"))
	v, _ := m.View("a.md")
	require.Equal(t, "foo BAZ foo", v)
}

func TestInsert(t *testing.T) {
	m := New(t.TempDir())
	require.NoError(t, m.Create("a.md", "line0\nline1"))
	require.NoError(t, m.Insert("a.md", 1, "INSERTED"))
	v, _ := m.View("a.md")
	require.Equal(t, "line0\nINSERTED\nline1", v)
}

func TestPathSafety(t *testing.T) {
	m := New(t.TempDir())
	require.Error(t, m.Create("../escape.md", "x"))
	_, err := m.View("../../etc/passwd")
	require.Error(t, err)
	require.Error(t, m.Delete("../../tmp/x"))
}

func TestRenameAndDelete(t *testing.T) {
	m := New(t.TempDir())
	require.NoError(t, m.Create("a.md", "x"))
	require.NoError(t, m.Rename("a.md", "b/c.md"))
	_, err := m.View("a.md")
	require.Error(t, err)
	v, _ := m.View("b/c.md")
	require.Equal(t, "x", v)
	require.NoError(t, m.Delete("b/c.md"))
	_, err = m.View("b/c.md")
	require.Error(t, err)
}
