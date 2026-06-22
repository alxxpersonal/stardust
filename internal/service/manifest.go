package service

import (
	"context"
	"path/filepath"

	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/manifest"
)

// RefreshManifest regenerates the pinned agent boot manifest from docs state.
func (s *Service) RefreshManifest(_ context.Context) error {
	groups, err := s.Registry([]string{"specs", "plans", "adr", "research"})
	if err != nil {
		return err
	}
	input := manifest.AgentManifestInput{
		VaultName:    filepath.Base(s.Layout.Root),
		RegistryPath: "docs/INDEX.md",
		IndexPath:    ".stardust/INDEX.md",
		ActivePlans:  activePlanRecords(groups, 5),
		StaleDocs:    staleDocRecords(s.Layout.Root, s.Config.Ignore, groups, 5),
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

func staleDocRecords(root string, ignore []string, groups []manifest.RegistryGroup, limit int) []manifest.RegistryRecord {
	issues, err := convention.CheckDocs(root, ignore)
	if err != nil {
		return nil
	}
	byPath := map[string]manifest.RegistryRecord{}
	for _, group := range groups {
		for _, record := range group.Records {
			byPath[record.Path] = record
		}
	}
	var out []manifest.RegistryRecord
	for _, issue := range issues {
		if issue.Kind != "stale-governed-doc" {
			continue
		}
		record, ok := byPath[issue.Path]
		if !ok {
			continue
		}
		out = append(out, record)
		if len(out) == limit {
			return out
		}
	}
	return out
}
