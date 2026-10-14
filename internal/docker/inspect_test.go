package docker

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
)

func TestParseInspect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    engine.ImageInfo
		wantErr string
	}{
		{
			name:  "labeled image",
			input: `[{"Id":"sha256:abc","Os":"linux","Architecture":"amd64","Config":{"Labels":{"k":"v","a":"b"}},"Extra":1}]`,
			want:  engine.ImageInfo{ID: "sha256:abc", OS: "linux", Arch: "amd64", Labels: map[string]string{"k": "v", "a": "b"}},
		},
		{
			name:  "null labels",
			input: `[{"Id":"sha256:abc","Os":"linux","Architecture":"arm64","Config":{"Labels":null}}]`,
			want:  engine.ImageInfo{ID: "sha256:abc", OS: "linux", Arch: "arm64"},
		},
		{
			name:  "first image wins",
			input: `[{"Id":"one"},{"Id":"two"}]`,
			want:  engine.ImageInfo{ID: "one"},
		},
		{name: "empty array", input: `[]`, wantErr: "parse docker image inspect output: no image in the reply"},
		{name: "not JSON", input: `Error`, wantErr: "parse docker image inspect output: invalid character 'E' looking for beginning of value"},
		{name: "wrong shape", input: `{"Id":"x"}`, wantErr: "parse docker image inspect output: json: cannot unmarshal object into Go value of type []struct { ID string \"json:\\\"Id\\\"\"; OS string \"json:\\\"Os\\\"\"; Architecture string; Config struct { Labels map[string]string } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseInspect([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("parseInspect() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseInspect() = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}
