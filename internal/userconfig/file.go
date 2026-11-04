package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// config is config.yaml as read and validated. Paths are kept as written;
// expand turns them into host paths once the home directory is known.
type config struct {
	source      string // the file, shown with ~, for messages
	raw         []byte // the bytes parsed; nil when the file is missing
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

// The lists are lists of pointers because yaml drops a null item (a bare ~, or
// an empty "- ") from a list without a word. A deny entry that vanishes
// silently would loosen the policy, so the validators refuse a null item: a
// null string is read as "" (orEmpty), a null root is refused by nullRoot.
type fileImage struct {
	Apt []*string `yaml:"apt"`
}

type fileLinks struct {
	Allow []*fileLinkRoot `yaml:"allow"`
	Deny  []*string       `yaml:"deny"`
}

type fileLinkRoot struct {
	Path     string `yaml:"path"`
	ReadOnly bool   `yaml:"read_only"`
}

type fileWorkdirs struct {
	Allow []*fileWorkdirRoot `yaml:"allow"`
}

type fileWorkdirRoot struct {
	Path string `yaml:"path"`
}

// parse validates the contents of a config.yaml. source names the file in
// error messages. An empty file is a valid, empty config.
func parse(data []byte, source string) (config, error) {
	file, err := decode(data, source)
	if err != nil {
		return config{}, err
	}
	return file.validate(source)
}

// decode reads the one YAML document in data, refusing unknown keys. An empty
// document decodes to the zero fileV1.
func decode(data []byte, source string) (fileV1, error) {
	var file fileV1
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		if errors.Is(err, io.EOF) {
			return fileV1{}, nil
		}
		return fileV1{}, decodeError(source, err)
	}
	var next yaml.Node
	if err := decoder.Decode(&next); !errors.Is(err, io.EOF) {
		return fileV1{}, fmt.Errorf("%s: only one YAML document is allowed", source)
	}
	if file.Workdirs == nil && hasWorkdirsKey(data) {
		file.Workdirs = &fileWorkdirs{} // a null value still restricts, to no root at all
	}
	return file, nil
}

// hasWorkdirsKey reports whether the document has a workdirs key. yaml leaves
// fileV1.Workdirs nil for a null value (a bare "workdirs:" or "workdirs: ~"),
// which must restrict like an empty list rather than read as no key.
func hasWorkdirsKey(data []byte) bool {
	var keys struct {
		Workdirs yaml.Node `yaml:"workdirs"` // set even for a null value
	}
	if err := yaml.Unmarshal(data, &keys); err != nil {
		// The strict decode in decode has already accepted data, so this cannot
		// happen; if it ever does, restrict rather than allow every directory.
		return true
	}
	return !keys.Workdirs.IsZero()
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
		return fmt.Errorf("%s: invalid YAML: %s", source, invalidYAML(err.Error()))
	}
	messages := make([]string, 0, len(typeErr.Errors))
	for _, message := range typeErr.Errors {
		messages = append(messages, friendly(message))
	}
	return fmt.Errorf("%s: %s", source, strings.Join(messages, "; "))
}

// friendly rewrites one yaml type error:
// "line 3: field imgae not found in type userconfig.fileV1" -> `line 3: unknown field "imgae"`,
// "line 3: field image already set in type userconfig.fileV1" -> `line 3: duplicate field "image"`,
// "line 2: cannot unmarshal !!str `x` into []string" -> "line 2: wrong type (found str)",
// "line 1: cannot unmarshal !foo `x` into userconfig.fileImage" -> "line 1: wrong type".
func friendly(message string) string {
	line, rest, ok := strings.Cut(message, ": ")
	if !ok {
		return message
	}
	if field, ok := strings.CutPrefix(rest, "field "); ok {
		if name, _, ok := strings.Cut(field, " not found in type "); ok {
			return fmt.Sprintf("%s: unknown field %q", line, name)
		}
		if name, _, ok := strings.Cut(field, " already set in type "); ok {
			return fmt.Sprintf("%s: duplicate field %q", line, name)
		}
	}
	if found, ok := strings.CutPrefix(rest, "cannot unmarshal "); ok {
		tag, _, _ := strings.Cut(found, " ")
		if kind, ok := strings.CutPrefix(tag, "!!"); ok && isKind(kind) {
			return fmt.Sprintf("%s: wrong type (found %s)", line, kind)
		}
		// Any other tag is the user's own text, which yaml shows in place of
		// the value's kind and may hold control characters: do not echo it.
		return line + ": wrong type"
	}
	return message
}

// invalidYAML returns yaml's message for input it cannot read, without its
// prefix. When an explicit core tag does not fit its value, yaml shows the
// value raw ("cannot decode !!str `x` as a !!int"); it is the user's text and
// may hold control characters, so it is quoted with %q, like field names.
func invalidYAML(message string) string {
	message = strings.TrimPrefix(message, "yaml: ")
	rest, ok := strings.CutPrefix(message, "cannot decode !!")
	if !ok {
		return message
	}
	from, value, found := strings.Cut(rest, " `")
	// the value may contain "` as a !!" itself; the tag it was decoded as comes last
	const as = "` as a !!"
	if i := strings.LastIndex(value, as); found && i >= 0 && isKind(from) && isKind(value[i+len(as):]) {
		return fmt.Sprintf("cannot decode %s %q as %s", from, value[:i], value[i+len(as):])
	}
	return "cannot decode a tagged value"
}

// isKind reports whether kind is a plain lowercase word, like the yaml core
// kinds str, seq and map.
func isKind(kind string) bool {
	return kind != "" && strings.Trim(kind, "abcdefghijklmnopqrstuvwxyz") == ""
}
