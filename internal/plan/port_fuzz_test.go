package plan

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// portForm is PORT or HOST_PORT:CONTAINER_PORT in decimal, the only form
// ParsePorts accepts (written independently of its parser).
var portForm = regexp.MustCompile(`^([0-9]+)(?::([0-9]+))?$`)

// expectPort returns the port value must parse to, or false when it must be
// refused: wrong form, or a number outside 1-65535.
func expectPort(value string) (Port, bool) {
	m := portForm.FindStringSubmatch(value)
	if m == nil {
		return Port{}, false
	}
	number := func(s string) (int, bool) {
		n, err := strconv.ParseUint(s, 10, 64)
		return int(n), err == nil && n >= 1 && n <= 65535
	}
	host, ok := number(m[1])
	if !ok {
		return Port{}, false
	}
	container := host
	if m[2] != "" {
		if container, ok = number(m[2]); !ok {
			return Port{}, false
		}
	}
	return Port{HostIP: "127.0.0.1", HostPort: host, ContainerPort: container}, true
}

// FuzzParsePorts checks ParsePorts against expectPort: one value parses to
// exactly the expected port or is refused with an error naming it; a valid
// port re-parses from its own text; a host port published twice is refused;
// and two valid ports keep their order.
func FuzzParsePorts(f *testing.F) {
	for _, seed := range [][2]string{
		{"3000", "8080:3000"}, {"03000", "1"}, {"65535:65535", "65536"}, {"0", "3000-3005"}, {"0.0.0.0:80:80", "3000/udp"},
		{"abc", ""}, {"8080:3000/tcp", ":3000"}, {"3000:", "99999999999999999999"}, {"+5", " 5"}, {"5", "5:6"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		want, valid := expectPort(a)
		got, err := ParsePorts([]string{a})
		if !valid {
			if err == nil || got != nil || !strings.HasPrefix(err.Error(), fmt.Sprintf("invalid port %q", a)) {
				t.Fatalf("ParsePorts([%q]) = %v, %v; want an error naming the value", a, got, err)
			}
			return
		}
		if err != nil || !reflect.DeepEqual(got, []Port{want}) {
			t.Fatalf("ParsePorts([%q]) = %v, %v; want %v", a, got, err, want)
		}
		text := fmt.Sprintf("%d:%d", want.HostPort, want.ContainerPort)
		if again, err := ParsePorts([]string{text}); err != nil || !reflect.DeepEqual(again, got) {
			t.Fatalf("ParsePorts([%q]) = %v, %v; want %v", text, again, err, got)
		}
		if _, err := ParsePorts([]string{a, a}); err == nil || err.Error() != fmt.Sprintf("host port %d is published more than once", want.HostPort) {
			t.Fatalf("ParsePorts([%q, %q]) error = %v", a, a, err)
		}
		if other, ok := expectPort(b); ok && other.HostPort != want.HostPort {
			if both, err := ParsePorts([]string{a, b}); err != nil || !reflect.DeepEqual(both, []Port{want, other}) {
				t.Fatalf("ParsePorts([%q, %q]) = %v, %v; want %v", a, b, both, err, []Port{want, other})
			}
		}
	})
}
