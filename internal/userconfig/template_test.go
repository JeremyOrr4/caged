package userconfig

import (
	"reflect"
	"strings"
	"testing"
)

func TestStarterConfigChangesNoDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := parse([]byte(starterConfig), testSource)
	if err != nil {
		t.Fatalf("parse(starterConfig) error = %v; want a valid empty config", err)
	}
	if !reflect.DeepEqual(cfg, config{}) {
		t.Errorf("parse(starterConfig) = %+v; want the empty config", cfg)
	}
}

func TestStarterConfigExamplesAreValidOnceUncommented(t *testing.T) {
	t.Parallel()

	var enabled strings.Builder
	for line := range strings.SplitSeq(starterConfig, "\n") {
		if strings.HasPrefix(line, "## ") || line == "##" {
			continue
		}
		enabled.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "#"), " ") + "\n")
		if rest, ok := strings.CutPrefix(line, "#"); ok && !strings.HasPrefix(rest, " ") && rest != "" {
			t.Fatalf("example line %q must start with \"# \"", line)
		}
	}
	cfg, err := parse([]byte(enabled.String()), testSource)
	if err != nil {
		t.Fatalf("uncommented examples do not parse: %v\n%s", err, enabled.String())
	}
	if len(cfg.apt) == 0 || len(cfg.linkAllow) == 0 || len(cfg.linkDeny) == 0 || !cfg.workdirsSet {
		t.Errorf("uncommented examples = %+v; want image.apt, links.allow, links.deny and workdirs set", cfg)
	}
}
