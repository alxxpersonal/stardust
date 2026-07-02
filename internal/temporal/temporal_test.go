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

func TestAnchorOf(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		ok       bool
		reversal bool
		cue      string
	}{
		{"assertion decided", "We decided to use Postgres for storage.", true, false, "decided"},
		{"assertion we will", "We will ship the digest feature.", true, false, "we will"},
		{"assertion canonical", "The canonical brand name is Stardust.", true, false, "canonical"},
		{"reversal no longer", "Postgres is no longer used here.", true, true, "no longer"},
		{"reversal deprecated", "This endpoint is deprecated.", true, true, "deprecated"},
		{"reversal contraction", "The old parser won't run anymore.", true, true, "won't"},
		{"reversal beats assertion", "We decided against Postgres.", true, true, "decided against"},
		{"plain prose", "The sky is blue today.", false, false, ""},
		{"empty", "   ", false, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ok := AnchorOf(tc.line)
			require.Equal(t, tc.ok, ok)
			if !tc.ok {
				return
			}
			require.Equal(t, tc.reversal, a.Reversal)
			require.Equal(t, tc.cue, a.Cue)
		})
	}
}
