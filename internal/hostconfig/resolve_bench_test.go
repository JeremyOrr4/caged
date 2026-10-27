package hostconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// countingFS counts the filesystem calls that dominate config resolution.
type countingFS struct {
	system.FS
	lstats, readDirs int
}

func (c *countingFS) Lstat(name string) (fs.FileInfo, error) {
	c.lstats++
	return c.FS.Lstat(name)
}

func (c *countingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	c.readDirs++
	return c.FS.ReadDir(name)
}

// fileNames returns n file names f0000... inside dir, relative to home.
func fileNames(dir string, n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = filepath.Join(dir, fmt.Sprintf("f%04d", i))
	}
	return names
}

// benchmarkResolve resolves home repeatedly with one agent whose ~/.claude is a
// split tree (no masks, not staged), and reports the filesystem calls of one Resolve.
func benchmarkResolve(b *testing.B, home string) {
	b.Helper()
	fsys := &countingFS{}
	agents := []agent.Agent{{Name: "bench", Command: []string{"bench"}, ConfigPaths: []agent.ConfigPath{{HostPath: ".claude", ContainerPath: ".claude", Writable: true}}}}
	resolver := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agents, Policies: policies{}})
	for b.Loop() {
		if _, err := resolver.Resolve(home); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(fsys.lstats)/float64(b.N), "lstat/op")
	b.ReportMetric(float64(fsys.readDirs)/float64(b.N), "readdir/op")
}

// BenchmarkResolveWideTree resolves a config tree of 5,000 files in 50
// directories, symlink-free (mounted whole) and with one symlink (split).
// cmd/caged's BenchmarkPlan plans on the same split tree.
func BenchmarkResolveWideTree(b *testing.B) {
	for _, split := range []bool{false, true} {
		b.Run(fmt.Sprintf("split=%v", split), func(b *testing.B) {
			home := b.TempDir()
			for d := range 50 {
				writeFiles(b, home, fileNames(filepath.Join(".claude", fmt.Sprintf("d%02d", d)), 100)...)
			}
			if split {
				symlink(b, home, ".claude/d00/link", "f0000")
			}
			benchmarkResolve(b, home)
		})
	}
}

// BenchmarkResolveDeepTree resolves a chain of nested directories holding
// 5,000 files spread over the levels, with a symlink at the bottom so that no
// level is symlink-free. The nested directory sorts before (a-sub) or after
// (z-sub) the files, which changes how often each level is re-inspected (F25).
func BenchmarkResolveDeepTree(b *testing.B) {
	for _, depth := range []int{5, 50, 500} {
		for _, sub := range []string{"a-sub", "z-sub"} {
			b.Run(fmt.Sprintf("depth=%d/subdir=%s", depth, sub), func(b *testing.B) {
				home := b.TempDir()
				dir := ".claude"
				for range depth {
					writeFiles(b, home, fileNames(dir, 5000/depth)...)
					dir = filepath.Join(dir, sub)
				}
				symlink(b, home, filepath.Join(dir, "link"), "../f0000")
				benchmarkResolve(b, home)
			})
		}
	}
}
