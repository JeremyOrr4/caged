package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/configstage"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// RunContainer starts an interactive, disposable container and waits for it to
// exit. Staged config mounts run from copies that are written back when the
// container exits. A non-zero container exit is returned unchanged so its
// code can be passed on. When caged is signalled during the run the container
// may still be running, so the copies are removed without being written back
// (D-25) and the *system.SignalError is returned.
func (c *Client) RunContainer(ctx context.Context, spec plan.RunSpec) error {
	docker, err := c.dockerPath()
	if err != nil {
		return err
	}
	session, err := c.stager.Stage(spec.ConfigMounts)
	if err != nil {
		return err
	}
	spec.ConfigMounts = session.Mounts()
	mask, err := c.createMaskFile(spec.ConfigMounts)
	if err != nil {
		return errors.Join(err, session.Cleanup())
	}
	defer c.removeMaskFile(mask)
	return finish(session, c.run(ctx, docker, spec, mask.path))
}

// finish ends the staging session of a run that returned runErr: it writes
// the copies back and removes them, or after a signal only removes them,
// whether caged caught it or it ended the docker CLI, which leaves the
// container possibly still writing.
func finish(session *configstage.Session, runErr error) error {
	if errors.As(runErr, new(*system.SignalError)) || errors.As(runErr, new(*system.KilledError)) {
		if session.Copied() {
			runErr = fmt.Errorf("%w; Pi config changes from this run were not written back", runErr)
		}
		if err := session.Cleanup(); err != nil {
			return errors.Join(runErr, err)
		}
		return runErr
	}
	if err := errors.Join(session.Sync(), session.Cleanup()); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr // not joined with nil: errors.Join would wrap it
}

// run runs `docker run` for spec and explains a failure where it can.
func (c *Client) run(ctx context.Context, docker string, spec plan.RunSpec, maskSource string) error {
	var stderr bytes.Buffer
	err := c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   runArgs(spec, c.platform, maskSource),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: io.MultiWriter(c.stdio.Err, &stderr),
		// caged outlives a signal so it can remove the staged copies
		CatchSignals: true,
	})
	if err != nil {
		return classifyRunError(c.platform, spec, stderr.String(), err)
	}
	return nil
}

// maskFile is the empty file mounted over masked config files. The zero
// value is no file: an empty path, and nothing to remove.
type maskFile struct{ dir, path string }

// createMaskFile creates the mask file in a new temporary directory when any
// mount has a file mask, and returns the zero maskFile when none does.
func (c *Client) createMaskFile(mounts []plan.ConfigMount) (maskFile, error) {
	if !hasFileMasks(mounts) {
		return maskFile{}, nil
	}
	dir, err := c.fs.MkdirTemp("", "caged-run-masks-")
	if err != nil {
		return maskFile{}, fmt.Errorf("create config mask source: %w", err)
	}
	mask := maskFile{dir: dir, path: filepath.Join(dir, "empty-file")}
	if err := c.fs.WriteFile(mask.path, nil, 0o600); err != nil {
		c.removeMaskFile(mask)
		return maskFile{}, fmt.Errorf("create config mask file: %w", err)
	}
	return mask, nil
}

// removeMaskFile removes the directory of mask. The zero maskFile has none,
// so RunContainer can defer it without checking.
func (c *Client) removeMaskFile(mask maskFile) {
	if mask.dir == "" {
		return
	}
	_ = c.fs.RemoveAll(mask.dir) // best effort: a leftover empty file in the temp dir is harmless
}

func hasFileMasks(mounts []plan.ConfigMount) bool {
	for _, mount := range mounts {
		if len(mount.MaskFiles) > 0 {
			return true
		}
	}
	return false
}
