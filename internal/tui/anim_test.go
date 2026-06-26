package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alxxpersonal/stardust/internal/tui/anim"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// The pure animation, color, border, and bar helpers moved to internal/tui/anim;
// their unit tests live there. What stays here covers the tui-specific
// composition: the App frame loop and the titled animated box.

// --- Frame Sync Test ---

func TestAppSyncsFrameToSearchTab(t *testing.T) {
	app := newApp(nil)
	model, _ := app.Update(anim.FlameTickMsg{})
	updated, ok := model.(App)
	assert.True(t, ok)
	assert.Equal(t, 1, updated.frame, "flame tick should advance the global frame")
	assert.Equal(t, 1, updated.searchTab.frame, "the search tab frame must stay in lockstep")
}

// --- Animated Box Tests ---

func TestAnimatedBoxWrapsContent(t *testing.T) {
	out := animatedBox("hello", 0)
	assert.Contains(t, components.SanitizeText(out), "hello")
	assert.Contains(t, out, "╭", "the box should use a rounded border")
}

func TestAnimatedBoxRotatesBorder(t *testing.T) {
	assert.NotEqual(t, animatedBox("x", 0), animatedBox("x", 8),
		"the border gradient should rotate every eight frames")
}
