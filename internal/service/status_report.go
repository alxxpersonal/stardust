package service

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/gitx"
)

// --- Status report ---

// VaultStatus is the full state probe for one directory: whether it is
// initialized, its detected kind, its collections with live record counts, the
// derived index health, and an init hint when uninitialized.
type VaultStatus struct {
	Root        string           `json:"root"`
	Initialized bool             `json:"initialized"`
	Kind        string           `json:"kind"`
	Repository  RepositoryInfo   `json:"repository"`
	Collections []CollectionInfo `json:"collections"`
	Index       IndexHealth      `json:"index"`
	Source      SourceBinding    `json:"source"`
	Hint        string           `json:"hint,omitempty"`
}

// SourceBinding is the source-repo root that cross-repo wiki-to-code drift binds
// against, with its origin. Path is empty when nothing is bound; Origin is
// "configured" for an explicit source_root or "detected" for an autodetected
// sibling checkout.
type SourceBinding struct {
	Path   string `json:"path,omitempty"`
	Origin string `json:"origin,omitempty"` // configured | detected
}

// RepositoryInfo is the git repository context for a status report.
type RepositoryInfo struct {
	IsGit  bool   `json:"is_git"`
	Name   string `json:"name,omitempty"`
	Branch string `json:"branch,omitempty"`
	Head   string `json:"head_sha,omitempty"`
}

// IndexHealth is the derived-index portion of a status report: indexed note
// count, whether vectors are live (with the reason when off), whether a reranker
// is active (with its announced source), how far behind HEAD the index sits
// (when git is available), the last indexed commit, and the embed model.
type IndexHealth struct {
	Notes            int    `json:"notes"`
	Vectors          bool   `json:"vectors"`
	VectorsReason    string `json:"vectors_reason,omitempty"`
	Reranker         bool   `json:"reranker"`
	RerankerSource   string `json:"reranker_source,omitempty"`
	CommitsBehind    int    `json:"commits_behind"`
	HasCommitsBehind bool   `json:"has_commits_behind"`
	LastIndexed      string `json:"last_indexed_sha,omitempty"`
	EmbedModel       string `json:"embed_model,omitempty"`
}

// GatherStatus resolves the vault root from start and reports full state. When
// no .stardust is found it returns an uninitialized report (detected kind plus
// an init hint) and a nil error, so "not initialized" is a normal result, not an
// error. Otherwise it opens the service, composes the existing index-health,
// collections, and freshness reads plus the detected kind, and closes.
func GatherStatus(ctx context.Context, start string) (VaultStatus, error) {
	root, err := config.FindRoot(start)
	if err != nil {
		if errors.Is(err, config.ErrNoVault) {
			kind, _ := convention.DetectKind(start)
			return VaultStatus{
				Root:        start,
				Initialized: false,
				Kind:        kind.Label(),
				Repository:  repositoryInfo(ctx, start),
				Collections: []CollectionInfo{},
				Hint:        "run stardust init to initialize this directory",
			}, nil
		}
		return VaultStatus{}, err
	}

	svc, err := Open(ctx, root)
	if err != nil {
		return VaultStatus{}, err
	}
	defer func() { _ = svc.Close() }()

	st, err := svc.Status(ctx)
	if err != nil {
		return VaultStatus{}, err
	}
	cols, err := svc.ListCollections(ctx)
	if err != nil {
		return VaultStatus{}, err
	}
	if cols == nil {
		cols = []CollectionInfo{}
	}
	behind, hasBehind := svc.commitsBehindHead(ctx)
	kind, _ := convention.DetectKind(root)

	reason := ""
	if !st.Vectors {
		reason = ftsOnlyReason
	}

	sourcePath, sourceOrigin, err := convention.ResolveSourceRoot(svc.Config, root)
	if err != nil {
		return VaultStatus{}, err
	}

	return VaultStatus{
		Root:        root,
		Initialized: true,
		Kind:        kind.Label(),
		Repository:  repositoryInfo(ctx, root),
		Collections: cols,
		Index: IndexHealth{
			Notes:            st.Notes,
			Vectors:          st.Vectors,
			VectorsReason:    reason,
			Reranker:         st.Reranker,
			RerankerSource:   st.RerankerSource,
			CommitsBehind:    behind,
			HasCommitsBehind: hasBehind,
			LastIndexed:      st.LastIndexed,
			EmbedModel:       st.EmbedModel,
		},
		Source: SourceBinding{Path: sourcePath, Origin: sourceOrigin},
	}, nil
}

func repositoryInfo(ctx context.Context, root string) RepositoryInfo {
	if !gitx.IsRepo(ctx, root) {
		return RepositoryInfo{}
	}
	branch, _ := gitx.Branch(ctx, root)
	head, _ := gitx.HeadSHA(ctx, root)
	return RepositoryInfo{
		IsGit:  true,
		Name:   filepath.Base(root),
		Branch: branch,
		Head:   head,
	}
}
