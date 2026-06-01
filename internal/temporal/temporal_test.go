package temporal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommitments(t *testing.T) {
	body := "# Heading\nsome prose\nTODO: fix the parser\nI'll send the report tomorrow\n- [ ] write tests\nnormal line\nFIXME later"
	c := Commitments("a.md", body)
	require.Len(t, c, 4) // TODO, I'll, unchecked box, FIXME; heading and prose skipped
	require.Equal(t, "a.md", c[0].Path)
	require.Contains(t, c[0].Line, "fix the parser")
}

func TestTopArea(t *testing.T) {
	require.Equal(t, "projects", TopArea("projects/x/note.md"))
	require.Equal(t, "(root)", TopArea("note.md"))
}
