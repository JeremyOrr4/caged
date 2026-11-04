package configstage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// These tests pin how write-back handles conflicts: the host version is
// kept, every conflict is reported, and one failure does not stop the rest.

func TestStageRefusesConflictingChanges(t *testing.T) {
	tests := []struct {
		name      string
		container func(t *testing.T, stage string)
		host      func(t *testing.T, root string)
		want      string
		hostFile  string
		hostData  string
	}{
		{
			name:      "host deleted a file the container edited",
			container: func(t *testing.T, stage string) { writeTestFile(t, filepath.Join(stage, "a.json"), "container", 0o600) },
			host: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "a.json")); err != nil {
					t.Fatal(err)
				}
			},
			want: "disappeared",
		},
		{
			name: "host edited a file the container deleted",
			container: func(t *testing.T, stage string) {
				if err := os.Remove(filepath.Join(stage, "a.json")); err != nil {
					t.Fatal(err)
				}
			},
			host:     func(t *testing.T, root string) { writeTestFile(t, filepath.Join(root, "a.json"), "host", 0o600) },
			want:     "both changed",
			hostFile: "a.json", hostData: "host",
		},
		{
			name: "both added the same file differently",
			container: func(t *testing.T, stage string) {
				writeTestFile(t, filepath.Join(stage, "new.json"), "container", 0o600)
			},
			host:     func(t *testing.T, root string) { writeTestFile(t, filepath.Join(root, "new.json"), "host", 0o600) },
			want:     "both changed",
			hostFile: "new.json", hostData: "host",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, mount := piTree(t)
			staged, persist := stageTree(t, []plan.ConfigMount{mount})
			tc.container(t, staged[0].HostPath)
			tc.host(t, root)
			if err := persist(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("persist() error = %v, want it to contain %q", err, tc.want)
			}
			if tc.hostFile != "" {
				if got := readTestFile(t, filepath.Join(root, tc.hostFile)); got != tc.hostData {
					t.Fatalf("%s = %q, want the host version %q", tc.hostFile, got, tc.hostData)
				}
			}
		})
	}
}

func TestStageAcceptsTheSameAddOnBothSides(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	writeTestFile(t, filepath.Join(staged[0].HostPath, "new.json"), "same", 0o600)
	writeTestFile(t, filepath.Join(root, "new.json"), "same", 0o600)
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(root, "new.json")); got != "same" {
		t.Fatalf("new.json = %q", got)
	}
}

func TestStageDoesNotOverwriteConcurrentHostChange(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "agent", "settings.json")
	if err := os.WriteFile(settings, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	mounts := []plan.ConfigMount{{Staged: true, Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"agent/blocked-link"}}}
	staged, persist := stageTree(t, mounts)
	if err := os.WriteFile(filepath.Join(staged[0].HostPath, "agent", "settings.json"), []byte("container edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("host edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err == nil || !strings.Contains(err.Error(), "both changed") {
		t.Fatalf("persist() error = %v, want concurrent modification conflict", err)
	}
	if got, err := os.ReadFile(settings); err != nil || string(got) != "host edit" {
		t.Fatalf("host's concurrent edit = %q, %v", got, err)
	}
}

func TestStageRefusesAHostDirectoryThatDisappeared(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.RemoveAll(filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staged[0].HostPath, "sub", "c.json"), "container", 0o600)
	want := persistError(root, "host config directory disappeared during container run: "+filepath.Join(root, "sub"))
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	assertMissing(t, filepath.Join(root, "sub"))
}

func TestStageRefusesConflictingDirectoryModes(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.Chmod(filepath.Join(staged[0].HostPath, "sub"), 0o710); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	want := persistError(root, "host and container both changed permissions on "+filepath.Join(root, "sub")+"; leaving the host version intact")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if info, err := os.Lstat(filepath.Join(root, "sub")); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("sub: info=%v err=%v, want the host's mode 0750", info, err)
	}
}

func TestStageKeepsADirectoryTheHostAddedTo(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	writeTestFile(t, filepath.Join(root, "sub", "new.json"), "new", 0o600)
	if err := os.RemoveAll(filepath.Join(staged[0].HostPath, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(root, "sub", "c.json"))
	if got := readTestFile(t, filepath.Join(root, "sub", "new.json")); got != "new" {
		t.Fatalf("sub/new.json = %q, want the host's new file kept", got)
	}
}

func TestStageRefusesADeletedDirectoryTheHostChanged(t *testing.T) {
	tests := []struct {
		name      string
		host      func(t *testing.T, root string)
		container func(t *testing.T, stage string)
		entry     string
	}{
		{
			name: "the host changed the permissions of a directory the container deleted",
			host: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, "sub"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			container: func(t *testing.T, stage string) {
				if err := os.RemoveAll(filepath.Join(stage, "sub")); err != nil {
					t.Fatal(err)
				}
			},
			entry: "sub",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, mount := piTree(t)
			staged, persist := stageTree(t, []plan.ConfigMount{mount})
			tc.host(t, root)
			tc.container(t, staged[0].HostPath)
			want := persistError(root, "host config changed during container run at "+filepath.Join(root, tc.entry))
			if err := persist(); err == nil || err.Error() != want {
				t.Fatalf("persist() error = %v, want %q", err, want)
			}
			if _, err := os.Lstat(filepath.Join(root, tc.entry)); err != nil {
				t.Fatalf("host %s: %v, want it kept", tc.entry, err)
			}
		})
	}
}

// B-CS-1: a failed entry does not end the tree's write-back. The container's
// other changes still reach the host, and every failure is reported, each on
// its own line.
func TestWriteBackContinuesPastAFailedEntry(t *testing.T) {
	root, mount := piTree(t)
	writeTestFile(t, filepath.Join(root, "sub", "d.json"), "d", 0o600)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	for _, name := range []string{"a.json", "b.json", "sub/c.json", "sub/d.json"} {
		writeTestFile(t, filepath.Join(staged[0].HostPath, filepath.FromSlash(name)), "container", 0o600)
	}
	writeTestFile(t, filepath.Join(staged[0].HostPath, "z.json"), "new", 0o600)
	var want []string
	for _, name := range []string{"a.json", "sub/c.json", "sub/d.json"} {
		host := filepath.Join(root, filepath.FromSlash(name))
		writeTestFile(t, host, "host", 0o600)
		want = append(want, persistError(root, "host and container both changed "+host+"; leaving the host version intact"))
	}
	if err := persist(); err == nil || err.Error() != strings.Join(want, "\n") {
		t.Fatalf("persist() error = %v, want %q", err, strings.Join(want, "\n"))
	}
	for name, content := range map[string]string{"a.json": "host", "b.json": "container", "sub/c.json": "host", "sub/d.json": "host", "z.json": "new"} {
		if got := readTestFile(t, filepath.Join(root, filepath.FromSlash(name))); got != content {
			t.Errorf("%s = %q, want %q", name, got, content)
		}
	}
}

// B-CS-1: a deletion that conflicts does not stop the container's other
// deletions or its writes.
func TestWriteBackContinuesPastAFailedDeletion(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	for _, name := range []string{"a.json", "b.json"} {
		if err := os.Remove(filepath.Join(staged[0].HostPath, name)); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(staged[0].HostPath, "sub", "c.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(root, "a.json"), "host", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "a.json")+"; leaving the host version intact")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, filepath.Join(root, "a.json")); got != "host" {
		t.Errorf("a.json = %q, want the host version", got)
	}
	assertMissing(t, filepath.Join(root, "b.json"))
	if got := readTestFile(t, filepath.Join(root, "sub", "c.json")); got != "container" {
		t.Errorf("sub/c.json = %q, want the container version", got)
	}
}

// B-CS-1: inside a directory the container deleted, a conflicting entry is
// kept with its directory, and the other entries are still removed.
func TestWriteBackContinuesPastAFailedDeletionInADeletedDirectory(t *testing.T) {
	root, mount := piTree(t)
	writeTestFile(t, filepath.Join(root, "sub", "d.json"), "d", 0o600)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.RemoveAll(filepath.Join(staged[0].HostPath, "sub")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "sub", "c.json"), "host", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "sub", "c.json")+"; leaving the host version intact")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, filepath.Join(root, "sub", "c.json")); got != "host" {
		t.Errorf("sub/c.json = %q, want the host version", got)
	}
	assertMissing(t, filepath.Join(root, "sub", "d.json"))
}

// B-CS-1: a directory that fails is skipped with everything inside it, and
// write-back goes on with its siblings.
func TestWriteBackSkipsTheContentsOfAFailedDirectory(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	writeTestFile(t, filepath.Join(staged[0].HostPath, "sub", "c.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(staged[0].HostPath, "sub", "d.json"), "new", 0o600)
	writeTestFile(t, filepath.Join(staged[0].HostPath, "b.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(staged[0].HostPath, "z.json"), "new", 0o600)
	if err := os.Chmod(filepath.Join(root, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(staged[0].HostPath, "sub"), 0o710); err != nil {
		t.Fatal(err)
	}
	want := persistError(root, "host and container both changed permissions on "+filepath.Join(root, "sub")+"; leaving the host version intact")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, filepath.Join(root, "sub", "c.json")); got != "c" {
		t.Errorf("sub/c.json = %q, want the host version", got)
	}
	assertMissing(t, filepath.Join(root, "sub", "d.json"))
	for name, content := range map[string]string{"b.json": "container", "z.json": "new"} {
		if got := readTestFile(t, filepath.Join(root, name)); got != content {
			t.Errorf("%s = %q, want %q", name, got, content)
		}
	}
}

func TestStageRefusesADeletedDirectoryHoldingAHostEdit(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.RemoveAll(filepath.Join(staged[0].HostPath, "sub")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "sub", "c.json"), "host", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "sub", "c.json")+"; leaving the host version intact")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, filepath.Join(root, "sub", "c.json")); got != "host" {
		t.Fatalf("sub/c.json = %q, want the host version", got)
	}
}
