package agentsync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const managedMarkerName = ".stardust-sync-managed"

// Apply executes create and repair actions from a sync plan.
func Apply(plan Plan) (Plan, error) {
	for _, action := range plan.Actions {
		if err := applyAction(action, plan.Repair); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func applyAction(action Action, repair bool) error {
	switch action.Status {
	case "ok":
		return nil
	case "create":
		return createTarget(action)
	case "drift":
		if !repair {
			return fmt.Errorf("sync drift at %s; rerun with --repair", action.Target)
		}
		if err := os.RemoveAll(action.Target); err != nil {
			return fmt.Errorf("remove drifted target %s: %w", action.Target, err)
		}
		return createTarget(action)
	case "conflict":
		if !repair {
			return fmt.Errorf("sync conflict at %s; rerun with --repair after review", action.Target)
		}
		if !isManagedTarget(action.Target) {
			return fmt.Errorf("sync conflict at %s is not stardust-managed", action.Target)
		}
		if err := os.RemoveAll(action.Target); err != nil {
			return fmt.Errorf("remove managed conflict %s: %w", action.Target, err)
		}
		return createTarget(action)
	default:
		return fmt.Errorf("unsupported sync action status %q for %s", action.Status, action.Target)
	}
}

func createTarget(action Action) error {
	switch action.Mode {
	case "", "symlink":
		return createSymlink(action.Source, action.Target)
	case "copy":
		return copyPath(action.Source, action.Target)
	default:
		return fmt.Errorf("unsupported sync mode %q for %s", action.Mode, action.Target)
	}
}

func createSymlink(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create target parent %s: %w", filepath.Dir(target), err)
	}
	link := source
	if rel, err := filepath.Rel(filepath.Dir(target), source); err == nil {
		link = rel
	}
	if err := os.Symlink(link, target); err != nil {
		return fmt.Errorf("symlink %s to %s: %w", target, source, err)
	}
	return nil
}

func copyPath(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat copy source %s: %w", source, err)
	}
	if info.IsDir() {
		if err := copyDir(source, target); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, managedMarkerName), []byte("stardust\n"), 0o644); err != nil {
			return fmt.Errorf("write managed marker %s: %w", target, err)
		}
		return nil
	}
	if err := copyFile(source, target, info.Mode()); err != nil {
		return err
	}
	marker := target + "." + managedMarkerName
	if err := os.WriteFile(marker, []byte("stardust\n"), 0o644); err != nil {
		return fmt.Errorf("write managed marker %s: %w", marker, err)
	}
	return nil
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("rel copy path %s: %w", path, err)
		}
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return fmt.Errorf("create copy dir %s: %w", dst, err)
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat copy file %s: %w", path, err)
		}
		return copyFile(path, dst, info.Mode())
	})
}

func copyFile(source, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create copy parent %s: %w", filepath.Dir(target), err)
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open copy source %s: %w", source, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return fmt.Errorf("create copy target %s: %w", target, err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s to %s: %w", source, target, err)
	}
	return nil
}

func isManagedTarget(target string) bool {
	if _, err := os.Stat(filepath.Join(target, managedMarkerName)); err == nil {
		return true
	}
	if _, err := os.Stat(target + "." + managedMarkerName); err == nil {
		return true
	}
	return false
}
