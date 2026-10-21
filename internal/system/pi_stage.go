package system

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PiConfigMount is the filesystem portion of a Pi config mount. The system
// adapter stages these entries so Docker never resolves a mount destination
// through a host config symlink.
type PiConfigMount struct {
	Root, HostPath, DestPath string
	MaskPaths, MaskFiles     []string
	Link                     string
}

// StagePiMounts makes symlink-free copies of Pi config mounts. Docker resolves
// a nested mount destination through a symlink in a parent bind mount, so
// mounting child masks/targets directly can escape the config destination.
// The staged trees preserve the resolver's mount plan and are synchronized
// back to their host sources after the container exits. It returns the staged
// mounts, a cleanup callback, and a persistence callback.
func StagePiMounts(mounts []PiConfigMount) ([]PiConfigMount, func() error, func() error, error) {
	var roots []int
	for i, mount := range mounts {
		// A config root has no enclosing Pi mount. Nested link-target mounts
		// are associated with the nearest such root below.
		parent := false
		for j, other := range mounts {
			if i != j && other.Root == mount.Root && withinContainer(mount.DestPath, other.DestPath) {
				parent = true
				break
			}
		}
		if !parent {
			roots = append(roots, i)
		}
	}
	if len(roots) == 0 {
		return mounts, func() error { return nil }, func() error { return nil }, nil
	}

	var activeRoots []int
	for _, ri := range roots {
		root := mounts[ri]
		needs := len(root.MaskPaths) > 0 || len(root.MaskFiles) > 0
		for i, m := range mounts {
			if i != ri && m.Root == root.Root && withinContainer(m.DestPath, root.DestPath) {
				needs = true
			}
		}
		if needs {
			activeRoots = append(activeRoots, ri)
		}
	}
	if len(activeRoots) == 0 {
		return mounts, func() error { return nil }, func() error { return nil }, nil
	}

	tempRoot, err := os.MkdirTemp("", "caged-pi-config-")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create Pi config staging directory: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(tempRoot) }
	staged := append([]PiConfigMount(nil), mounts...)
	var syncItems []piSyncItem
	stagedRootNo := 0
	for _, rootIndex := range activeRoots {
		rootMount := mounts[rootIndex]
		group := []int{rootIndex}
		for i, m := range mounts {
			if i != rootIndex && m.Root == rootMount.Root && withinContainer(m.DestPath, rootMount.DestPath) {
				group = append(group, i)
			}
		}
		sort.Slice(group, func(i, j int) bool {
			return pathDepth(mounts[group[i]].DestPath) < pathDepth(mounts[group[j]].DestPath)
		})
		for _, mountIndex := range group {
			mount := mounts[mountIndex]
			stagedRootNo++
			target := filepath.Join(tempRoot, fmt.Sprintf("mount-%d", stagedRootNo))
			children := nestedMountPaths(mounts, group, mountIndex)
			masks := append(append([]string(nil), mount.MaskPaths...), mount.MaskFiles...)
			if err := copySafeTree(mount.HostPath, target, masks, children); err != nil {
				_ = cleanup()
				return nil, nil, nil, fmt.Errorf("stage Pi config %s: %w", mount.DestPath, err)
			}
			staged[mountIndex].HostPath = target
			staged[mountIndex].MaskPaths = nil
			staged[mountIndex].MaskFiles = nil
			var childDest []string
			for _, childIndex := range group {
				if childIndex == mountIndex || !withinContainer(mounts[childIndex].DestPath, mount.DestPath) {
					continue
				}
				if rel, ok := containerRelative(mount.DestPath, mounts[childIndex].DestPath); ok {
					childDest = append(childDest, filepath.ToSlash(rel))
				}
			}
			baseline, err := snapshotSafeTree(target, masks, childDest)
			if err != nil {
				_ = cleanup()
				return nil, nil, nil, fmt.Errorf("snapshot Pi config %s: %w", mount.HostPath, err)
			}
			syncItems = append(syncItems, piSyncItem{source: mount.HostPath, staged: target, masks: masks, children: childDest, baseline: baseline})
		}
		// Parent staging trees contain empty mountpoints for child targets.
		// Each child's source is a separate temporary tree, never a path inside
		// the parent source bind.
		for _, parentIndex := range group {
			parentStage := staged[parentIndex].HostPath
			for _, childIndex := range group {
				if childIndex == parentIndex || !withinContainer(mounts[childIndex].DestPath, mounts[parentIndex].DestPath) {
					continue
				}
				intermediate := false
				for _, candidate := range group {
					if candidate != parentIndex && candidate != childIndex && withinContainer(mounts[candidate].DestPath, mounts[parentIndex].DestPath) && withinContainer(mounts[childIndex].DestPath, mounts[candidate].DestPath) {
						intermediate = true
						break
					}
				}
				if intermediate {
					continue
				}
				rel, _ := containerRelative(mounts[parentIndex].DestPath, mounts[childIndex].DestPath)
				placeholder := filepath.Join(parentStage, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(placeholder), 0o700); err != nil {
					_ = cleanup()
					return nil, nil, nil, fmt.Errorf("create Pi config staging mountpoint: %w", err)
				}
				childInfo, err := os.Lstat(mounts[childIndex].HostPath)
				if err != nil {
					_ = cleanup()
					return nil, nil, nil, fmt.Errorf("inspect Pi config mount source %s: %w", mounts[childIndex].HostPath, err)
				}
				if childInfo.IsDir() {
					err = os.MkdirAll(placeholder, 0o700)
				} else {
					err = os.WriteFile(placeholder, nil, 0o600)
				}
				if err != nil {
					_ = cleanup()
					return nil, nil, nil, fmt.Errorf("create Pi config staging mountpoint: %w", err)
				}
			}
		}
	}
	return staged, cleanup, func() error {
		var errs []error
		for _, item := range syncItems {
			if err := syncSafeTree(item.staged, item.source, item.masks, item.children, item.baseline, "."); err != nil {
				errs = append(errs, fmt.Errorf("persist Pi config %s: %w", item.source, err))
			}
		}
		return errorsJoin(errs)
	}, nil
}

type piSyncItem struct {
	source, staged  string
	masks, children []string
	baseline        map[string]piFileState
}

type piFileState struct {
	dir    bool
	mode   fs.FileMode
	digest [32]byte
}

func nestedMountPaths(mounts []PiConfigMount, group []int, parent int) []string {
	var paths []string
	for _, child := range group {
		if child == parent || !withinContainer(mounts[child].DestPath, mounts[parent].DestPath) {
			continue
		}
		if rel, ok := containerRelative(mounts[parent].DestPath, mounts[child].DestPath); ok {
			paths = append(paths, filepath.ToSlash(rel))
		}
	}
	return paths
}

func copySafeTree(source, target string, masks, children []string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return copyRegular(source, target, info.Mode())
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a regular file or directory")
	}
	if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chmod(target, info.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if path == source {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if withinAny(slash, masks) || withinAny(slash, children) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if walkErr != nil {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		out := filepath.Join(target, rel)
		if info.IsDir() {
			if err := os.MkdirAll(out, info.Mode().Perm()); err != nil {
				return err
			}
			return os.Chmod(out, info.Mode().Perm())
		}
		if info.Mode().IsRegular() {
			return copyRegular(path, out, info.Mode())
		}
		return nil
	})
}

func snapshotSafeTree(source string, masks, children []string) (map[string]piFileState, error) {
	states := make(map[string]piFileState)
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		digest, err := fileDigest(source)
		if err != nil {
			return nil, err
		}
		states["."] = piFileState{digest: digest, mode: info.Mode().Perm()}
		return states, nil
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source is not a regular file or directory")
	}
	states["."] = piFileState{dir: true, mode: info.Mode().Perm()}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if path == source {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if withinAny(slash, masks) || withinAny(slash, children) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if walkErr != nil {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			states[slash] = piFileState{dir: true, mode: info.Mode().Perm()}
		} else if info.Mode().IsRegular() {
			digest, err := fileDigest(path)
			if err != nil {
				return err
			}
			states[slash] = piFileState{digest: digest, mode: info.Mode().Perm()}
		}
		return nil
	})
	return states, err
}

func syncSafeTree(staged, source string, masks, children []string, baseline map[string]piFileState, rel string) error {
	stageInfo, err := os.Lstat(staged)
	if err != nil {
		return err
	}
	if stageInfo.Mode().IsRegular() {
		stageHash, err := fileDigest(staged)
		if err != nil {
			return err
		}
		old, existed := baseline[rel]
		if existed && old.dir {
			return fmt.Errorf("host config changed type during container run at %s", rel)
		}
		if existed && old.digest == stageHash && old.mode == stageInfo.Mode().Perm() {
			return nil // the container did not change this file
		}
		if current, err := os.Lstat(source); err == nil {
			if !current.Mode().IsRegular() || current.Mode()&fs.ModeSymlink != 0 {
				return nil // do not follow or replace a host-side link/type change
			}
			currentHash, err := fileDigest(source)
			if err != nil {
				return err
			}
			currentMode := current.Mode().Perm()
			if existed && (currentHash != old.digest || currentMode != old.mode) && (currentHash != stageHash || currentMode != stageInfo.Mode().Perm()) || !existed && (currentHash != stageHash || currentMode != stageInfo.Mode().Perm()) {
				return fmt.Errorf("host and container both changed %s; leaving the host version intact", source)
			}
		} else if !os.IsNotExist(err) {
			return err
		} else if existed {
			return fmt.Errorf("host config file disappeared during container run: %s", source)
		}
		return replaceRegular(staged, source, stageInfo.Mode())
	}
	if !stageInfo.IsDir() || stageInfo.Mode()&fs.ModeSymlink != 0 {
		return nil
	}
	old, existed := baseline[rel]
	sourceInfo, sourceErr := os.Lstat(source)
	if sourceErr == nil {
		if !sourceInfo.IsDir() || sourceInfo.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("host config changed type during container run at %s", rel)
		}
	} else if os.IsNotExist(sourceErr) {
		if existed {
			return fmt.Errorf("host config directory disappeared during container run: %s", source)
		}
		if err := ensureSafeDir(source, stageInfo.Mode().Perm()); err != nil {
			return err
		}
		sourceInfo, sourceErr = os.Lstat(source)
		if sourceErr != nil {
			return sourceErr
		}
	} else {
		return sourceErr
	}
	if existed && !old.dir {
		return fmt.Errorf("host config changed type during container run at %s", rel)
	}
	if existed && old.mode != stageInfo.Mode().Perm() {
		if sourceInfo.Mode().Perm() != old.mode && sourceInfo.Mode().Perm() != stageInfo.Mode().Perm() {
			return fmt.Errorf("host and container both changed permissions on %s; leaving the host version intact", source)
		}
		if sourceInfo.Mode().Perm() == old.mode {
			if err := os.Chmod(source, stageInfo.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	stageEntries, err := os.ReadDir(staged)
	if err != nil {
		return err
	}
	stageNames := make(map[string]bool, len(stageEntries))
	for _, entry := range stageEntries {
		stageNames[entry.Name()] = true
	}
	sourceEntries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range sourceEntries {
		name := entry.Name()
		if withinAny(name, masks) || withinAny(name, children) || stageNames[name] {
			continue
		}
		if err := removeBaselineEntry(filepath.Join(source, name), childPath(rel, name), baseline); err != nil {
			return err
		}
	}
	for _, entry := range stageEntries {
		name := entry.Name()
		if withinAny(name, masks) || withinAny(name, children) {
			continue
		}
		from, to := filepath.Join(staged, name), filepath.Join(source, name)
		stageEntry, err := os.Lstat(from)
		if err != nil || stageEntry.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		childRel := childPath(rel, name)
		if stageEntry.IsDir() {
			if err := syncSafeTree(from, to, stripPrefixLists(masks, name), stripPrefixLists(children, name), baseline, childRel); err != nil {
				return err
			}
		} else if stageEntry.Mode().IsRegular() {
			if err := syncSafeTree(from, to, nil, nil, baseline, childRel); err != nil {
				return err
			}
		}
	}
	return nil
}

// childPath is the slash path of name inside the tree directory rel, the key
// of the baseline; "." is the root of the tree.
func childPath(rel, name string) string {
	if rel == "." {
		return name
	}
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(rel), name))
}

func removeBaselineEntry(path, rel string, baseline map[string]piFileState) error {
	old, existed := baseline[rel]
	if !existed {
		return nil // added by the host while the container was running
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || info.Mode()&fs.ModeSymlink != 0 {
		return nil
	}
	if info.IsDir() {
		if !old.dir || info.Mode().Perm() != old.mode {
			return fmt.Errorf("host config changed during container run at %s", path)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := removeBaselineEntry(filepath.Join(path, entry.Name()), filepath.ToSlash(filepath.Join(filepath.FromSlash(rel), entry.Name())), baseline); err != nil {
				return err
			}
		}
		entries, err = os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return os.Remove(path)
		}
		return nil
	}
	if !info.Mode().IsRegular() || old.dir {
		return nil
	}
	current, err := fileDigest(path)
	if err != nil {
		return err
	}
	if current != old.digest || info.Mode().Perm() != old.mode {
		return fmt.Errorf("host and container both changed %s; leaving the host version intact", path)
	}
	return os.Remove(path)
}

func fileDigest(path string) ([32]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	if copyErr != nil {
		return [32]byte{}, copyErr
	}
	if closeErr != nil {
		return [32]byte{}, closeErr
	}
	var out [32]byte
	copy(out[:], digest.Sum(nil))
	return out, nil
}

func ensureSafeDir(path string, mode fs.FileMode) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("destination is no longer a directory")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.Mkdir(path, mode); err != nil && !os.IsExist(err) {
		return err
	} else if err == nil {
		if err := os.Chmod(path, mode.Perm()); err != nil {
			return err
		}
	}
	return ensureSafeDir(path, mode)
}

func copyRegular(source, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	from, err := os.Open(source)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		_ = from.Close()
		return err
	}
	_, copyErr := io.Copy(out, from)
	fromErr := from.Close()
	if copyErr == nil {
		copyErr = fromErr
	}
	if copyErr == nil {
		copyErr = out.Chmod(mode.Perm())
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func replaceRegular(source, target string, mode fs.FileMode) error {
	if err := ensureSafeDir(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil && (!info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0) {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".caged-sync-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	from, err := os.Open(source)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, from)
	fromErr := from.Close()
	if copyErr == nil {
		copyErr = fromErr
	}
	if copyErr == nil {
		copyErr = tmp.Chmod(mode.Perm())
	}
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if info, err := os.Lstat(target); err == nil && (!info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0) {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpName, target)
}

func withinAny(path string, roots []string) bool {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	for _, root := range roots {
		root = filepath.ToSlash(filepath.Clean(filepath.FromSlash(root)))
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func stripPrefixLists(paths []string, prefix string) []string {
	var out []string
	prefix = filepath.ToSlash(filepath.Clean(filepath.FromSlash(prefix)))
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		if path == prefix {
			out = append(out, ".")
		} else if strings.HasPrefix(path, prefix+"/") {
			out = append(out, strings.TrimPrefix(path, prefix+"/"))
		}
	}
	return out
}

func withinContainer(child, parent string) bool {
	rel, ok := containerRelative(parent, child)
	return ok && rel != "."
}

func containerRelative(parent, child string) (string, bool) {
	parent, child = filepath.ToSlash(filepath.Clean(parent)), filepath.ToSlash(filepath.Clean(child))
	if child == parent {
		return ".", true
	}
	if !strings.HasPrefix(child, strings.TrimSuffix(parent, "/")+"/") {
		return "", false
	}
	return strings.TrimPrefix(child, strings.TrimSuffix(parent, "/")+"/"), true
}

func pathDepth(path string) int { return strings.Count(filepath.ToSlash(path), "/") }

func errorsJoin(errs []error) error {
	return errors.Join(errs...)
}
