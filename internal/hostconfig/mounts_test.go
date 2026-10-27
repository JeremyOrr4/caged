package hostconfig

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

func TestDropNestedLinkMountsKeepsRootLinksAndStagedMounts(t *testing.T) {
	t.Parallel()
	tree := plan.ConfigMount{Agent: "a", HostPath: "/h/.a", DestPath: "/home/caged/.a", Root: "/h/.a"}
	nested := plan.ConfigMount{Agent: "a", HostPath: "/h/x", DestPath: "/home/caged/.a/x", Root: "/h/.a", Link: "/h/.a/x"}
	rootLink := plan.ConfigMount{Agent: "b", HostPath: "/h/y", DestPath: "/home/caged/.a/y", Root: "/h/.a/y", Link: "/h/.a/y"}
	staged := plan.ConfigMount{Agent: "c", HostPath: "/h/z", DestPath: "/home/caged/.a/z", Root: "/h/.c", Link: "/h/.c/z", Staged: true}
	beside := plan.ConfigMount{Agent: "a", HostPath: "/h/w", DestPath: "/home/caged/.aw", Root: "/h/.a", Link: "/h/.a/w"}
	r := &resolution{report: plan.ConfigReport{Mounts: []plan.ConfigMount{tree, nested, rootLink, staged, beside}}}

	r.dropNestedLinkMounts()

	if want := []plan.ConfigMount{tree, rootLink, staged, beside}; !reflect.DeepEqual(r.report.Mounts, want) {
		t.Fatalf("Mounts = %+v, want %+v", r.report.Mounts, want)
	}
	want := []string{"/h/.a/x (symlink target would be mounted inside another mount at /home/caged/.a/x)"}
	if got := refusedStrings(r.report.Refused); !reflect.DeepEqual(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
}

func TestRepeatedMissingAndRefusedPathsAreReportedOnce(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".shared/settings.json", "private/secret")
	symlink(t, home, ".shared/private", "../private")
	paths := []agent.ConfigPath{{HostPath: ".absent", ContainerPath: ".absent"}, {HostPath: ".shared", ContainerPath: ".shared", Writable: true}}
	agents := []agent.Agent{{Name: "one", Command: []string{"one"}, ConfigPaths: paths}, {Name: "two", Command: []string{"two"}, ConfigPaths: paths}}

	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agents, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "" +
		"mount one ~/.shared/settings.json -> /home/caged/.shared/settings.json root=~/.shared writable=true staged=false masks=[] files=[] link=\n" +
		"missing ~/.absent\n" +
		"refused ~/.shared/private (symlink target ~/private is not under links.allow); add \"- path: ~/private\" under links.allow in ~/.config/caged/config.yaml\n"
	if got := describeReport(report, home, resolved); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}
