package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/api"
	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	mountsDir := config.Layout{Root: root}.Mounts()
	require.NoError(t, os.MkdirAll(filepath.Join(mountsDir, "gmail"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mountsDir, "gmail", "config.toml"),
		[]byte("command = \"gmail-mcp\"\ntool = \"search\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"),
		[]byte("---\ntitle: Test Note\n---\n# Test Note\nhello world content about gardening, see [[other]]"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.md"),
		[]byte("---\ntitle: Other\n---\n# Other\nlinked from note, see [[note]]"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}

func TestAPI_Health(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz") //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPI_StatusAndQuery(t *testing.T) {
	srv := newTestServer(t)

	var st struct {
		Notes int `json:"notes"`
	}
	getJSON(t, srv.URL+"/status", &st)
	require.Equal(t, 2, st.Notes)

	var qr struct {
		Hits []struct {
			Path string `json:"path"`
		} `json:"hits"`
	}
	getJSON(t, srv.URL+"/query?q=gardening", &qr)
	require.NotEmpty(t, qr.Hits)
	require.Equal(t, "note.md", qr.Hits[0].Path)
}

func TestAPI_Note(t *testing.T) {
	srv := newTestServer(t)
	var n struct {
		Title       string `json:"title"`
		Body        string `json:"body"`
		LinkTargets []struct {
			Link string `json:"link"`
			Path string `json:"path"`
		} `json:"link_targets"`
	}
	getJSON(t, srv.URL+"/note?path=note.md", &n)
	require.Equal(t, "Test Note", n.Title)
	require.Contains(t, n.Body, "gardening")
	require.Len(t, n.LinkTargets, 1)
	require.Equal(t, "other", n.LinkTargets[0].Link)
	require.Equal(t, "other.md", n.LinkTargets[0].Path)
}

func TestAPI_Mounts(t *testing.T) {
	srv := newTestServer(t)
	var ms []struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Target string `json:"target"`
		Tool   string `json:"tool"`
	}
	getJSON(t, srv.URL+"/mounts", &ms)
	require.Len(t, ms, 1)
	require.Equal(t, "gmail", ms[0].Name)
	require.Equal(t, "mcp", ms[0].Kind)
	require.Equal(t, "gmail-mcp", ms[0].Target)
	require.Equal(t, "search", ms[0].Tool)
}

func TestAPI_GraphPageRank(t *testing.T) {
	srv := newTestServer(t)
	var rep struct {
		Notes    int `json:"notes"`
		PageRank []struct {
			Path  string  `json:"path"`
			Title string  `json:"title"`
			Score float64 `json:"score"`
		} `json:"pagerank"`
	}
	getJSON(t, srv.URL+"/graph", &rep)
	require.Equal(t, 2, rep.Notes)
	require.NotEmpty(t, rep.PageRank)
	require.NotEmpty(t, rep.PageRank[0].Path)
	require.Greater(t, rep.PageRank[0].Score, 0.0)
}

func TestAPI_QueryMissingParam(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/query") //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// writeJobsCollection writes a "jobs" collection schema under root and returns
// the server seeded with it.
func writeJobsCollection(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".stardust", "collections", "jobs")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	schema := "path = \"Jobs\"\n" +
		"description = \"job applications\"\n" +
		"[[fields]]\nname = \"company\"\ntype = \"string\"\nrequired = true\n" +
		"[[fields]]\nname = \"status\"\ntype = \"enum\"\nenum = [\"applied\", \"interview\", \"offer\"]\n" +
		"[[fields]]\nname = \"score\"\ntype = \"number\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(schema), 0o644))
}

func newRecordsServer(t *testing.T) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	writeJobsCollection(t, root)

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	t.Cleanup(srv.Close)
	return srv
}

func postJSON(t *testing.T, method, url, body string, v any) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body)) //nolint:noctx
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	if v != nil && resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
	}
	return resp
}

func TestAPI_Collections(t *testing.T) {
	srv := newRecordsServer(t)

	var cols []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Description string `json:"description"`
		Records     int    `json:"records"`
		Fields      []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	getJSON(t, srv.URL+"/collections", &cols)
	require.Len(t, cols, 1)
	require.Equal(t, "jobs", cols[0].Name)
	require.Equal(t, "Jobs", cols[0].Path)
	require.Equal(t, "job applications", cols[0].Description)
	require.Equal(t, 0, cols[0].Records)
	require.Len(t, cols[0].Fields, 3)
	require.Equal(t, "company", cols[0].Fields[0].Name)

	var one struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	getJSON(t, srv.URL+"/collection?name=jobs", &one)
	require.Equal(t, "jobs", one.Name)
	require.Equal(t, "Jobs", one.Path)
}

func TestAPI_RecordsLifecycle(t *testing.T) {
	srv := newRecordsServer(t)

	// Create two records.
	var acme struct {
		Path  string `json:"path"`
		Title string `json:"title"`
	}
	resp := postJSON(t, http.MethodPost, srv.URL+"/records",
		`{"collection":"jobs","fields":{"company":"Acme","status":"applied","score":7},"body":"first lead"}`, &acme)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	require.Equal(t, "Jobs/acme.md", acme.Path)

	resp = postJSON(t, http.MethodPost, srv.URL+"/records",
		`{"collection":"jobs","fields":{"company":"Globex","status":"interview","score":9},"body":"second lead"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// List with a filter + descending numeric sort.
	var list struct {
		Collection string `json:"collection"`
		Folder     string `json:"folder"`
		Records    []struct {
			Path        string         `json:"path"`
			Frontmatter map[string]any `json:"frontmatter"`
		} `json:"records"`
	}
	getJSON(t, srv.URL+"/records?collection=jobs&sort=-score", &list)
	require.Equal(t, "Jobs", list.Folder)
	require.Len(t, list.Records, 2)
	require.Equal(t, float64(9), list.Records[0].Frontmatter["score"]) // Globex sorts first
	require.Equal(t, float64(7), list.Records[1].Frontmatter["score"])

	getJSON(t, srv.URL+"/records?collection=jobs&where=status:eq:applied", &list)
	require.Len(t, list.Records, 1)
	require.Equal(t, "Acme", list.Records[0].Frontmatter["company"])

	// Numeric predicate compares numerically, not as text.
	getJSON(t, srv.URL+"/records?collection=jobs&where=score:gte:8", &list)
	require.Len(t, list.Records, 1)
	require.Equal(t, "Globex", list.Records[0].Frontmatter["company"])

	// Get the single record (with body).
	var rec struct {
		Title       string         `json:"title"`
		Body        string         `json:"body"`
		Frontmatter map[string]any `json:"frontmatter"`
	}
	getJSON(t, srv.URL+"/record?path=Jobs/acme.md", &rec)
	require.Contains(t, rec.Body, "first lead")
	require.Equal(t, "applied", rec.Frontmatter["status"])

	// Patch the status, then confirm the filter follows.
	var patched struct {
		Frontmatter map[string]any `json:"frontmatter"`
	}
	resp = postJSON(t, http.MethodPatch, srv.URL+"/record?path=Jobs/acme.md",
		`{"fields":{"status":"offer"}}`, &patched)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	require.Equal(t, "offer", patched.Frontmatter["status"])

	getJSON(t, srv.URL+"/records?collection=jobs&where=status:eq:offer", &list)
	require.Len(t, list.Records, 1)
	require.Equal(t, "Acme", list.Records[0].Frontmatter["company"])

	// Delete the record: it drops from the collection and 404s on direct get.
	delResp := postJSON(t, http.MethodDelete, srv.URL+"/record?path=Jobs/acme.md", "", nil)
	require.Equal(t, http.StatusOK, delResp.StatusCode)
	_ = delResp.Body.Close()

	getJSON(t, srv.URL+"/records?collection=jobs", &list)
	require.Len(t, list.Records, 1, "deleted record is gone")
	require.Equal(t, "Globex", list.Records[0].Frontmatter["company"])

	missing, mErr := http.Get(srv.URL + "/record?path=Jobs/acme.md") //nolint:gosec,noctx
	require.NoError(t, mErr)
	_ = missing.Body.Close()
	require.Equal(t, http.StatusNotFound, missing.StatusCode)
}

func TestAPI_RecordsBadRequests(t *testing.T) {
	srv := newRecordsServer(t)

	resp, err := http.Get(srv.URL + "/records") //nolint:gosec,noctx
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, err = http.Get(srv.URL + "/records?collection=jobs&where=bogus") //nolint:gosec,noctx
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = postJSON(t, http.MethodPost, srv.URL+"/records", `{"fields":{}}`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
}
