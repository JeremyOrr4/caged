package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagePiMountsKeepsDockerDestinationsSymlinkFreeAndPersistsWrites(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".pi")
	target := filepath.Join(home, "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "agent", "settings.json")
	if err := os.Symlink("../../dotfiles/settings.json", link); err != nil {
		t.Fatal(err)
	}
	mounts := []PiConfigMount{
		{Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"agent/settings.json"}},
		{Root: root, HostPath: target, DestPath: "/home/caged/.pi/agent/settings.json", Link: link},
	}
	staged, cleanup, persist, err := StagePiMounts(mounts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup staging directory: %v", err)
		}
	})
	if len(staged) != 2 || strings.Contains(staged[0].HostPath, root) || strings.Contains(staged[1].HostPath, target) || staged[0].HostPath == staged[1].HostPath {
		t.Fatalf("Pi mounts were not redirected to temporary staging sources: %+v", staged)
	}
	if staged[0].MaskFiles != nil || staged[1].MaskFiles != nil {
		t.Fatalf("staged mounts still request Docker mask mounts: %+v", staged)
	}
	placeholder, err := os.Lstat(filepath.Join(staged[0].HostPath, "agent", "settings.json"))
	if err != nil || !placeholder.Mode().IsRegular() || placeholder.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("root staging tree has no safe regular mountpoint: info=%v err=%v", placeholder, err)
	}
	if got, err := os.ReadFile(staged[1].HostPath); err != nil || string(got) != "initial" {
		t.Fatalf("staged linked file = %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(staged[0].HostPath, "agent", "keybindings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staged[0].HostPath, "agent", "themes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged[0].HostPath, "agent", "themes", "new.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged[1].HostPath, []byte("updated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "agent", "keybindings.json")); err != nil || string(got) != "{}" {
		t.Fatalf("new Pi config file on host = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "agent", "themes", "new.json")); err != nil || string(got) != "{}" {
		t.Fatalf("new Pi config directory/file on host = %q, %v", got, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "updated" {
		t.Fatalf("linked Pi config file on host = %q, %v", got, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink was altered: info=%v err=%v", info, err)
	}
}

func TestStagePiMountsDoesNotOverwriteConcurrentHostChange(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "agent", "settings.json")
	if err := os.WriteFile(settings, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	mounts := []PiConfigMount{{Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"agent/blocked-link"}}}
	staged, cleanup, persist, err := StagePiMounts(mounts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup staging directory: %v", err)
		}
	})
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

func TestStagePiMountsWritesNestedDeletionsBack(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{"sub/c.json": "gone", "sub/keep.json": "keep", "c.json": "top-level namesake"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mounts := []PiConfigMount{{Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"agent/blocked-link"}}}
	staged, cleanup, persist, err := StagePiMounts(mounts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup staging directory: %v", err)
		}
	})
	if err := os.Remove(filepath.Join(staged[0].HostPath, "sub", "c.json")); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "sub", "c.json")); !os.IsNotExist(err) {
		t.Fatalf("sub/c.json was deleted in the container but is still on the host (err %v)", err)
	}
	for file, want := range map[string]string{"sub/keep.json": "keep", "c.json": "top-level namesake"} {
		if got, err := os.ReadFile(filepath.Join(root, file)); err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want it untouched", file, got, err)
		}
	}
}
