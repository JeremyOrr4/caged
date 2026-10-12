package container

import (
	"reflect"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		image   string
		command []string
		want    []string
	}{
		{
			name:    "shell command",
			image:   "ubuntu:24.04",
			command: []string{"bash"},
			want:    []string{"run", "-it", "--rm", "ubuntu:24.04", "bash"},
		},
		{
			name:  "no command",
			image: "alpine:3.21",
			want:  []string{"run", "-it", "--rm", "alpine:3.21"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildArgs(tt.image, tt.command); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildArgs(%q, %q) = %q, want %q", tt.image, tt.command, got, tt.want)
			}
		})
	}
}
