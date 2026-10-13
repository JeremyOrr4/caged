package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/JeremyOrr4/caged"

// allowedImports lists the caged packages each package may import. It is the
// dependency direction documented in docs/architecture.md: policy (app) never
// imports an adapter, and agent and system import nothing from caged.
func allowedImports() map[string][]string {
	return map[string][]string{
		"cmd/caged":           {"internal/agent", "internal/app", "internal/cli", "internal/docker", "internal/hostconfig", "internal/project", "internal/system"},
		"internal/cli":        {"internal/agent", "internal/app", "internal/runtime"},
		"internal/app":        {"internal/agent", "internal/plan", "internal/runtime"},
		"internal/docker":     {"internal/plan", "internal/system"},
		"internal/hostconfig": {"internal/agent", "internal/plan"},
		"internal/project":    {"internal/plan"},
		"internal/runtime":    {"internal/plan"},
		"internal/plan":       {"internal/agent"},
		"internal/agent":      {},
		"internal/system":     {},
	}
}

// osPackages give direct access to the operating system.
var osPackages = map[string]bool{"os": true, "os/exec": true, "os/signal": true, "os/user": true, "syscall": true, "runtime": true}

// filepathIO are the path/filepath functions that read the filesystem.
var filepathIO = map[string]bool{"Abs": true, "EvalSymlinks": true, "Glob": true, "Walk": true, "WalkDir": true}

type sourceFile struct {
	pkg  string // directory relative to the module root, e.g. "internal/app"
	path string
	ast  *ast.File
}

// productionFiles parses every non-test Go file in the module.
func productionFiles(t *testing.T) []sourceFile {
	t.Helper()
	root := filepath.Join("..", "..")
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		files = append(files, sourceFile{pkg: filepath.ToSlash(rel), path: path, ast: parsed})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Go files found")
	}
	return files
}

func importPaths(file *ast.File) []string {
	var paths []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func TestPackageDependenciesPointInward(t *testing.T) {
	t.Parallel()

	allowed := allowedImports()
	for _, file := range productionFiles(t) {
		permitted, known := allowed[file.pkg]
		if !known {
			t.Errorf("%s: package %s is not in allowedImports; add it and document it in docs/architecture.md", file.path, file.pkg)
			continue
		}
		for _, path := range importPaths(file.ast) {
			if !strings.HasPrefix(path, modulePath+"/") {
				continue
			}
			target := strings.TrimPrefix(path, modulePath+"/")
			ok := false
			for _, p := range permitted {
				ok = ok || p == target
			}
			if !ok {
				t.Errorf("%s: %s must not import %s", file.path, file.pkg, target)
			}
		}
	}
}

func TestAllowedDependencyGraphHasNoCycles(t *testing.T) {
	t.Parallel()

	allowed := allowedImports()
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var visit func(pkg string, trail []string)
	visit = func(pkg string, trail []string) {
		switch state[pkg] {
		case visiting:
			t.Fatalf("dependency cycle: %s -> %s", strings.Join(trail, " -> "), pkg)
		case done:
			return
		}
		state[pkg] = visiting
		for _, dep := range allowed[pkg] {
			visit(dep, append(trail, pkg))
		}
		state[pkg] = done
	}
	pkgs := make([]string, 0, len(allowed))
	for pkg := range allowed {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		visit(pkg, nil)
	}
}

func TestOnlySystemTouchesTheOperatingSystem(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		adapter := file.pkg == "internal/system"
		if adapter {
			continue
		}
		for _, path := range importPaths(file.ast) {
			if osPackages[path] && file.pkg != "cmd/caged" {
				t.Errorf("%s imports %q; only internal/system (and the composition root) may", file.path, path)
			}
		}
		ast.Inspect(file.ast, func(node ast.Node) bool {
			call, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := call.X.(*ast.Ident); ok && pkg.Name == "filepath" && filepathIO[call.Sel.Name] {
				t.Errorf("%s calls filepath.%s, which reads the filesystem; use an injected FileSystem", file.path, call.Sel.Name)
			}
			return true
		})
	}
}

func TestNoPackageLevelMutableState(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		for _, decl := range file.ast.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			if embedsFile(gen) {
				continue // go:embed requires a var; the embedded strings are never assigned
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if file.pkg == "cmd/caged" && name.Name == "version" {
						continue // set by -ldflags "-X main.version=..." at link time
					}
					t.Errorf("%s: package-level var %s; use a constant, a function, or a field", file.path, name.Name)
				}
			}
		}
	}
}

func embedsFile(decl *ast.GenDecl) bool {
	if decl.Doc == nil {
		return false
	}
	for _, comment := range decl.Doc.List {
		if strings.HasPrefix(comment.Text, "//go:embed ") {
			return true
		}
	}
	return false
}
