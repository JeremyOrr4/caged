package userconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
)

// AllowLink adds path, an absolute host directory or file, to links.allow in
// config.yaml, read_only when readOnly is set, and returns the file's new
// content. The user has just agreed to it on the host, so the caller records
// that content as approved.
//
// The edit starts from the bytes this Store read, the ones the config guard
// reviewed, and is written only when the file on disk still holds exactly
// them, so a change an agent made in the meantime is never approved along
// with it. The new line is inserted as text, so comments and layout stay as
// they are, and the result must parse to the old settings plus this one
// entry and give a valid link policy, or nothing is written. A config.yaml
// that is itself a symlink is not edited: a container can replace the file
// in its mounted folder with a link to anywhere. When path is already allowed
// the content is returned unchanged. Every error says what to add by hand.
func (s *Store) AllowLink(path string, readOnly bool) ([]byte, error) {
	e, err := s.newEdit(path, readOnly)
	if err != nil {
		return nil, err
	}
	if done, err := e.checkEntry(); done || err != nil {
		return e.cfg.raw, err
	}
	perm, err := e.fileMode()
	if err != nil {
		return nil, err
	}
	updated, err := insertAllow(e.cfg.raw, e.entry, readOnly)
	if err != nil {
		return nil, e.manual("%v", err)
	}
	next, err := s.checkEdit(e.cfg, updated, e.entry, readOnly, e.home)
	if err != nil {
		return nil, e.manual("%v", err)
	}
	if err := e.write(updated, perm); err != nil {
		return nil, err
	}
	// Later questions answer from what was written, not from a new read, so
	// a change made after the write is still left for the guard to show.
	next.source, next.raw, next.found = e.cfg.source, updated, true
	s.cfg, s.err, s.done = next, nil, true
	return updated, nil
}

// linkEdit is one AllowLink call.
type linkEdit struct {
	s           *Store
	cfg         config // the file as read and reviewed
	file, home  string
	path, entry string // the host path, and as written in the file
	readOnly    bool
}

func (s *Store) newEdit(path string, readOnly bool) (*linkEdit, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}
	file, err := s.Path()
	if err != nil {
		return nil, err
	}
	home, err := s.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	home, path = filepath.Clean(home), filepath.Clean(path)
	return &linkEdit{s: s, cfg: cfg, file: file, home: home, path: path, entry: entryPath(home, path), readOnly: readOnly}, nil
}

// manual is an error that ends with the line to add by hand.
func (e *linkEdit) manual(format string, args ...any) error {
	return fmt.Errorf("%s; add \"- path: %s\" under links.allow in %s yourself", fmt.Sprintf(format, args...), e.entry, e.cfg.source)
}

// checkEntry reports done when the path is already allowed, and refuses a
// path that cannot be an entry or would change how an existing entry
// decides: the first allow root that holds a path decides whether it is
// read-only, and the new entry is inserted first.
func (e *linkEdit) checkEntry() (done bool, err error) {
	if !filepath.IsAbs(e.path) || e.entry == "~" {
		return false, fmt.Errorf("caged will not add %q to links.allow", e.path)
	}
	for _, root := range e.cfg.linkAllow {
		existing := expand(e.home, root.path)
		if existing == e.path {
			return true, nil
		}
		if root.readOnly != e.readOnly && pathpolicy.Within(existing, e.path) {
			return false, e.manual("it holds the links.allow entry %s, whose read_only setting differs", root.path)
		}
	}
	return false, nil
}

// fileMode returns the permissions to keep, refusing a config.yaml that is a
// symlink or not a regular file.
func (e *linkEdit) fileMode() (fs.FileMode, error) {
	info, err := e.s.fs.Lstat(e.file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return 0o600, nil
	case err != nil:
		return 0, e.manual("inspect %s: %v", e.cfg.source, err)
	case info.Mode()&fs.ModeSymlink != 0:
		return 0, e.manual("%s is a symlink, which caged does not edit", e.cfg.source)
	case !info.Mode().IsRegular():
		return 0, e.manual("%s is not a regular file", e.cfg.source)
	}
	return info.Mode().Perm(), nil
}

// write replaces the file with updated, but only while it still holds the
// reviewed bytes.
func (e *linkEdit) write(updated []byte, perm fs.FileMode) error {
	current, err := e.s.fs.ReadRegularFile(e.file, maxConfigSize)
	switch {
	case errors.Is(err, fs.ErrNotExist) && !e.cfg.found:
		if err := e.s.fs.MkdirAllPrivate(filepath.Dir(e.file)); err != nil {
			return e.manual("create %s: %v", filepath.Dir(e.cfg.source), err)
		}
	case err != nil:
		return e.manual("read %s: %v", e.cfg.source, err)
	case !e.cfg.found || !bytes.Equal(current, e.cfg.raw):
		return e.manual("%s changed after caged reviewed it, so nothing was written; run caged again to review the change", e.cfg.source)
	}
	if err := e.s.fs.ReplaceFile(e.file, updated, perm); err != nil {
		return e.manual("write %s: %v", e.cfg.source, err)
	}
	return nil
}

// entryPath is path as written in config.yaml: with "~/" when it is inside
// home.
func entryPath(home, path string) string {
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	if path == home {
		return "~"
	}
	return path
}

// checkEdit makes sure updated, the edited file, holds exactly the settings
// of cfg plus the allow entry, and that its link policy is valid, also under
// case folding, as the resolver builds it on macOS. It returns the parsed
// file.
func (s *Store) checkEdit(cfg config, updated []byte, entry string, readOnly bool, home string) (config, error) {
	next, err := parse(updated, cfg.source)
	if err != nil {
		return config{}, fmt.Errorf("the edited file would not parse: %w", err)
	}
	// The entry goes first: insertAllow puts it before the first item, and a
	// new entry never decides a path an existing one decided differently
	// (AllowLink refuses an entry that holds a root of the other kind).
	want := cfg
	want.linkAllow = append([]allowRoot{{path: entry, readOnly: readOnly}}, cfg.linkAllow...)
	if !reflect.DeepEqual(settingsOf(next), settingsOf(want)) {
		return config{}, errors.New("caged could not add the entry without changing other settings")
	}
	policy, err := s.linkPolicy(next, home)
	if err == nil {
		_, err = policy.FoldCase()
	}
	if err != nil {
		return config{}, fmt.Errorf("the entry would make the config invalid: %w", err)
	}
	return next, nil
}

// settingsOf is the part of cfg that comes from the file's settings, with
// empty lists as nil, for comparison.
func settingsOf(cfg config) config {
	nilIfEmpty := func(list []string) []string {
		if len(list) == 0 {
			return nil
		}
		return list
	}
	out := config{apt: nilIfEmpty(cfg.apt), linkDeny: nilIfEmpty(cfg.linkDeny), workdirsSet: cfg.workdirsSet, workdirs: nilIfEmpty(cfg.workdirs)}
	if len(cfg.linkAllow) > 0 {
		out.linkAllow = cfg.linkAllow
	}
	return out
}

// insertAllow returns raw with one links.allow item for entry inserted as
// text, as the first entry. With no links key a block is appended to the
// end. Otherwise the item goes before the first item of a block-style allow
// list, under a bare "allow:" or "links:", or before the first key of the
// links mapping. Flow style ([], {}) and other layouts are refused rather
// than rewritten.
func insertAllow(raw []byte, entry string, readOnly bool) ([]byte, error) {
	quoted, err := quote(entry)
	if err != nil {
		return nil, err
	}
	item := func(dash int) []string {
		lines := []string{strings.Repeat(" ", dash) + "- path: " + quoted}
		if readOnly {
			lines = append(lines, strings.Repeat(" ", dash+2)+"read_only: true")
		}
		return lines
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot read the file to edit it: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return appendBlock(raw, item(4)), nil
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode || top.Style&yaml.FlowStyle != 0 {
		return nil, errors.New("the file is not a block-style mapping, which caged does not edit")
	}
	key, links := mappingValue(top, "links")
	if links == nil {
		return appendBlock(raw, item(4)), nil
	}
	return insertIntoLinks(strings.SplitAfter(string(raw), "\n"), key, links, item)
}

// insertIntoLinks inserts the item made by item into the links mapping,
// whose key is key, in lines.
func insertIntoLinks(lines []string, key, links *yaml.Node, item func(dash int) []string) ([]byte, error) {
	allowBlock := func(indent int) []string {
		return append([]string{strings.Repeat(" ", indent) + "allow:"}, item(indent+2)...)
	}
	if isBareNull(links) {
		return insertLines(lines, key.Line, allowBlock(key.Column-1+2)), nil
	}
	if links.Kind != yaml.MappingNode || links.Style&yaml.FlowStyle != 0 || len(links.Content) == 0 {
		return nil, errors.New("links is not a block-style mapping, which caged does not edit")
	}
	allowKey, allow := mappingValue(links, "allow")
	switch {
	case allow == nil:
		first := links.Content[0]
		return insertLines(lines, first.Line-1, allowBlock(first.Column-1)), nil
	case isBareNull(allow):
		return insertLines(lines, allowKey.Line, item(allowKey.Column-1+2)), nil
	case allow.Kind != yaml.SequenceNode || allow.Style&yaml.FlowStyle != 0 || len(allow.Content) == 0:
		return nil, errors.New("links.allow is not a block-style list, which caged does not edit")
	}
	first := allow.Content[0]
	dash, ok := dashColumn(lines, first)
	if !ok {
		return nil, errors.New("links.allow has a layout caged does not edit")
	}
	return insertLines(lines, first.Line-1, item(dash)), nil
}

// dashColumn returns the column, from 0, of the "-" that starts the list
// item node, when only blanks surround it on the line before the item.
func dashColumn(lines []string, node *yaml.Node) (int, bool) {
	if node.Line < 1 || node.Line > len(lines) {
		return 0, false
	}
	text := lines[node.Line-1]
	prefix := text[:min(len(text), max(node.Column-1, 0))]
	dash := strings.LastIndex(prefix, "-")
	if dash < 0 || strings.TrimSpace(prefix[:dash]) != "" || strings.TrimSpace(prefix[dash+1:]) != "" {
		return 0, false
	}
	return dash, true
}

// mappingValue returns the key and value nodes of name in the mapping m, or
// nils.
func mappingValue(m *yaml.Node, name string) (key, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == name {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

// isBareNull reports whether node is a null written as nothing at all ("key:").
func isBareNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null" && node.Value == ""
}

// insertLines returns lines with added inserted after the first after lines.
func insertLines(lines []string, after int, added []string) []byte {
	var out strings.Builder
	for i, line := range lines {
		if i == after {
			if i > 0 && !strings.HasSuffix(lines[i-1], "\n") {
				out.WriteString("\n")
			}
			writeLines(&out, added)
		}
		out.WriteString(line)
	}
	if after >= len(lines) {
		if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
			out.WriteString("\n")
		}
		writeLines(&out, added)
	}
	return []byte(out.String())
}

func writeLines(out *strings.Builder, lines []string) {
	for _, line := range lines {
		out.WriteString(line + "\n")
	}
}

// appendBlock appends a links.allow block holding item to raw.
func appendBlock(raw []byte, item []string) []byte {
	var out strings.Builder
	out.Write(raw)
	if len(raw) > 0 && !bytes.HasSuffix(raw, []byte("\n")) {
		out.WriteString("\n")
	}
	if len(raw) > 0 {
		out.WriteString("\n")
	}
	out.WriteString("## Added by caged when you allowed this link target.\nlinks:\n  allow:\n")
	writeLines(&out, item)
	return []byte(out.String())
}

// quote writes s as a YAML double-quoted scalar; JSON strings are valid YAML.
func quote(s string) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(s); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
