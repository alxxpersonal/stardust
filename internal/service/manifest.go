package service

import (
	"context"
	"path/filepath"

	"github.com/alxxpersonal/stardust/internal/manifest"
)

// RefreshManifest regenerates the pinned agent boot manifest from docs state.
func (s *Service) RefreshManifest(ctx context.Context) error {
	groups, err := s.Registry([]string{"specs", "plans", "adr", "research"})
	if err != nil {
		return err
	}
	stale, err := s.staleDocRecords(ctx, 5)
	if err != nil {
		return err
	}
	drift, err := s.driftDocRecords(ctx, 5)
	if err != nil {
		return err
	}
	input := manifest.AgentManifestInput{
		VaultName:    filepath.Base(s.Layout.Root),
		RegistryPath: "docs/INDEX.md",
		IndexPath:    ".stardust/INDEX.md",
		ActivePlans:  activePlanRecords(groups, 5),
		StaleDocs:    stale,
		DriftDocs:    drift,
	}
	return manifest.WriteAgentManifest(s.Layout.Manifest(), input)
}

func activePlanRecords(groups []manifest.RegistryGroup, limit int) []manifest.RegistryRecord {
	var out []manifest.RegistryRecord
	for _, group := range groups {
		if group.Name != "plans" {
			continue
		}
		for _, record := range group.Records {
			if record.Status == "Active" {
				out = append(out, record)
				if len(out) == limit {
					return out
				}
			}
		}
	}
	return out
}

// staleDocRecords sources the boot manifest's stale-doc rows from the registry
// stale query (the same StaleDocs scan that backs `registry stale`) instead of
// re-deriving a bare count via CheckDocs. This keeps the rich drift detail
// (changed-commit count and the moved files) that the manifest renders.
func (s *Service) staleDocRecords(ctx context.Context, limit int) ([]manifest.StaleDoc, error) {
	res, err := s.StaleDocs(ctx)
	if err != nil {
		return nil, err
	}
	var out []manifest.StaleDoc
	for _, doc := range res.Docs {
		out = append(out, manifest.StaleDoc{
			Title:          doc.Title,
			Path:           doc.DocPath,
			ChangedCommits: doc.ChangedCommits,
			Matched:        doc.Matched,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// driftDocRecords sources the boot manifest's reference-drift rows from the
// ungated DriftDocs scan (related: and inline code-path bindings), so a doc that
// trails its referenced code surfaces at boot regardless of an Implemented
// status it never carries.
func (s *Service) driftDocRecords(ctx context.Context, limit int) ([]manifest.DriftDoc, error) {
	res, err := s.DriftDocs(ctx)
	if err != nil {
		return nil, err
	}
	var out []manifest.DriftDoc
	for _, doc := range res.Docs {
		bindings := make([]manifest.DriftBinding, len(doc.Bindings))
		for i, bind := range doc.Bindings {
			bindings[i] = manifest.DriftBinding{File: bind.File, ChangedCommits: bind.ChangedCommits}
		}
		out = append(out, manifest.DriftDoc{Title: doc.Title, Path: doc.DocPath, Bindings: bindings})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}
