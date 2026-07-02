package rerank_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoCGOTransitiveDependency guards the pure-Go, CGO-free, single-static-binary
// release property (four goreleaser tarballs plus a brew formula, all built with
// CGO_ENABLED=0). It walks every transitive dependency of internal/rerank via
// `go list` and fails if any package carries cgo source files, so a future edit
// cannot silently pull a native ONNX runtime or a cgo tokenizer into the default
// build. Reranking must stay a pure net/http call to a discovered or configured
// endpoint, never an in-process inference runtime.
func TestNoCGOTransitiveDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-json",
		"github.com/alxxpersonal/stardust/internal/rerank").Output()
	require.NoError(t, err, "go list must succeed to audit the dependency graph")

	type pkg struct {
		ImportPath string
		Standard   bool
		CgoFiles   []string
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p pkg
		require.NoError(t, dec.Decode(&p))
		require.Emptyf(t, p.CgoFiles,
			"package %s pulls cgo (%v) into internal/rerank; the CGO-free release property forbids it",
			p.ImportPath, p.CgoFiles)
	}
}
