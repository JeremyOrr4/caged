package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// config is the validated content of config.yaml. Paths are kept as written;
// expand turns them into host paths once the home directory is known.
type config struct {
	found       bool
	apt         []string
	linkAllow   []allowRoot
	linkDeny    []string
	workdirsSet bool // the workdirs key is present, even with an empty list
	workdirs    []string
}

type allowRoot struct {
	path     string
	readOnly bool
}

// The file* types mirror config.yaml. Their Go names never reach users:
// decodeError rewrites yaml's messages.
type fileV1 struct {
	Version  *int          `yaml:"version"`
	Image    fileImage     `yaml:"image"`
	Links    fileLinks     `yaml:"links"`
	Workdirs *fileWorkdirs `yaml:"workdirs"`
}

// The string lists are lists of pointers because yaml drops a null item (a bare
// ~, or an empty "- ") from a []string without a word. A deny entry that
// vanishes silently would loosen the policy, so a null item is read as "" and
// the validators refuse it.
type fileImage struct {
	Apt []*string `yaml:"apt"`
}

type fileLinks struct {
	Allow []fileLinkRoot `yaml:"allow"`
	Deny  []*string      `yaml:"deny"`
}

type fileLinkRoot struct {
	Path     string `yaml:"path"`
	ReadOnly bool   `yaml:"read_only"`
}

type fileWorkdirs struct {
	Allow []fileWorkdirRoot `yaml:"allow"`
}

type fileWorkdirRoot struct {
	Path string `yaml:"path"`
}

// parse validates the contents of a config.yaml. source names the file in
// error messages. An empty file is a valid, empty config.
func parse(data []byte, source string) (config, error) {
	var file fileV1
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		if errors.Is(err, io.EOF) {
			return config{}, nil
		}
		return config{}, decodeError(source, err)
	}
	var next yaml.Node
	if err := decoder.Decode(&next); !errors.Is(err, io.EOF) {
		return config{}, fmt.Errorf("%s: only one YAML document is allowed", source)
	}
	if file.Version != nil && *file.Version != 1 {
		return config{}, fmt.Errorf("%s: unsupported version %d; this caged reads version 1", source, *file.Version)
	}
	apt := orEmpty(file.Image.Apt)
	for _, name := range apt {
		if !validAptName(name) {
			return config{}, fmt.Errorf("%s: invalid apt package name %q in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$", source, name)
		}
	}
	cfg := config{apt: sortedUnique(apt)}
	for _, root := range file.Links.Allow {
		if err := checkPath(source, "links.allow", root.Path); err != nil {
			return config{}, err
		}
		cfg.linkAllow = append(cfg.linkAllow, allowRoot{path: root.Path, readOnly: root.ReadOnly})
	}
	for _, path := range orEmpty(file.Links.Deny) {
		if err := checkPath(source, "links.deny", path); err != nil {
			return config{}, err
		}
		cfg.linkDeny = append(cfg.linkDeny, path)
	}
	if file.Workdirs != nil {
		cfg.workdirsSet = true
		for _, root := range file.Workdirs.Allow {
			if err := checkPath(source, "workdirs.allow", root.Path); err != nil {
				return config{}, err
			}
			cfg.workdirs = append(cfg.workdirs, root.Path)
		}
	}
	return cfg, nil
}

// orEmpty returns the values of list, with a null item as "".
func orEmpty(list []*string) []string {
	values := make([]string, 0, len(list))
	for _, item := range list {
		if item == nil {
			values = append(values, "")
			continue
		}
		values = append(values, *item)
	}
	return values
}

// decodeError turns a yaml failure into a message that names the file and
// the YAML, never caged's Go types.
func decodeError(source string, err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Errorf("%s: invalid YAML: %s", source, strings.TrimPrefix(err.Error(), "yaml: "))
	}
	messages := make([]string, 0, len(typeErr.Errors))
	for _, message := range typeErr.Errors {
		messages = append(messages, friendly(message))
	}
	return fmt.Errorf("%s: %s", source, strings.Join(messages, "; "))
}

// friendly rewrites one yaml type error:
// "line 3: field imgae not found in type userconfig.fileV1" -> `line 3: unknown field "imgae"`,
// "line 2: cannot unmarshal !!str `x` into []string" -> "line 2: wrong type (found str)".
func friendly(message string) string {
	line, rest, ok := strings.Cut(message, ": ")
	if !ok {
		return message
	}
	if field, ok := strings.CutPrefix(rest, "field "); ok {
		if name, _, ok := strings.Cut(field, " not found in type "); ok {
			return fmt.Sprintf("%s: unknown field %q", line, name)
		}
	}
	if found, ok := strings.CutPrefix(rest, "cannot unmarshal !!"); ok {
		kind, _, _ := strings.Cut(found, " ")
		return fmt.Sprintf("%s: wrong type (found %s)", line, kind)
	}
	return message
}
