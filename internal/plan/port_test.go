package plan

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	t.Parallel()

	const syntax = `; use PORT or HOST_PORT:CONTAINER_PORT, e.g. -p 3000 or -p 8080:3000`
	tests := []struct {
		name    string
		values  []string
		want    []Port
		wantErr string
	}{
		{name: "none", values: nil, want: nil},
		{name: "one port", values: []string{"3000"}, want: []Port{{HostIP: "127.0.0.1", HostPort: 3000, ContainerPort: 3000}}},
		{name: "host and container", values: []string{"8080:3000"}, want: []Port{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000}}},
		{name: "leading zeros", values: []string{"03000"}, want: []Port{{HostIP: "127.0.0.1", HostPort: 3000, ContainerPort: 3000}}},
		{name: "bounds", values: []string{"1", "65535:65535"}, want: []Port{{HostIP: "127.0.0.1", HostPort: 1, ContainerPort: 1}, {HostIP: "127.0.0.1", HostPort: 65535, ContainerPort: 65535}}},
		{
			name: "order is kept", values: []string{"8080:3000", "5173"},
			want: []Port{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000}, {HostIP: "127.0.0.1", HostPort: 5173, ContainerPort: 5173}},
		},
		{
			name: "same container port on two host ports", values: []string{"8080:3000", "8081:3000"},
			want: []Port{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000}, {HostIP: "127.0.0.1", HostPort: 8081, ContainerPort: 3000}},
		},

		{name: "empty", values: []string{""}, wantErr: `invalid port ""` + syntax},
		{name: "letters", values: []string{"abc"}, wantErr: `invalid port "abc"` + syntax},
		{name: "digit and letter", values: []string{"30a0"}, wantErr: `invalid port "30a0"` + syntax},
		{name: "missing host port", values: []string{":3000"}, wantErr: `invalid port ":3000"` + syntax},
		{name: "missing container port", values: []string{"3000:"}, wantErr: `invalid port "3000:"` + syntax},
		{name: "sign", values: []string{"+80"}, wantErr: `invalid port "+80"` + syntax},
		{name: "space", values: []string{" 80"}, wantErr: `invalid port " 80"` + syntax},
		{name: "lone dash", values: []string{"-1"}, wantErr: `invalid port "-1"` + syntax},
		{name: "trailing dash", values: []string{"3000-"}, wantErr: `invalid port "3000-"` + syntax},

		{name: "zero", values: []string{"0"}, wantErr: `invalid port "0": 0 is out of range 1-65535`},
		{name: "too high", values: []string{"70000"}, wantErr: `invalid port "70000": 70000 is out of range 1-65535`},
		{name: "zero container port", values: []string{"8080:0"}, wantErr: `invalid port "8080:0": 0 is out of range 1-65535`},
		{name: "huge", values: []string{"99999999999999999999"}, wantErr: `invalid port "99999999999999999999": 99999999999999999999 is out of range 1-65535`},
		{name: "high container port", values: []string{"80:65536"}, wantErr: `invalid port "80:65536": 65536 is out of range 1-65535`},

		{name: "range", values: []string{"3000-3005"}, wantErr: `invalid port "3000-3005": port ranges are not supported`},
		{name: "range on both sides", values: []string{"8000-8005:3000-3005"}, wantErr: `invalid port "8000-8005:3000-3005": port ranges are not supported`},
		{name: "range on one side", values: []string{"8080:3000-3005"}, wantErr: `invalid port "8080:3000-3005": port ranges are not supported`},

		{name: "any address", values: []string{"0.0.0.0:80:80"}, wantErr: `invalid port "0.0.0.0:80:80": a host address is not supported; caged always binds 127.0.0.1`},
		{name: "loopback address", values: []string{"127.0.0.1:80:80"}, wantErr: `invalid port "127.0.0.1:80:80": a host address is not supported; caged always binds 127.0.0.1`},
		{name: "IPv6 address", values: []string{"[::1]:80:80"}, wantErr: `invalid port "[::1]:80:80": a host address is not supported; caged always binds 127.0.0.1`},

		{name: "udp", values: []string{"3000/udp"}, wantErr: `invalid port "3000/udp": protocols are not supported; caged publishes TCP only`},
		{name: "tcp", values: []string{"3000/tcp"}, wantErr: `invalid port "3000/tcp": protocols are not supported; caged publishes TCP only`},
		{name: "mapping with protocol", values: []string{"8080:3000/udp"}, wantErr: `invalid port "8080:3000/udp": protocols are not supported; caged publishes TCP only`},

		{name: "duplicate host port", values: []string{"3000", "3000:4000"}, wantErr: "host port 3000 is published more than once"},
		{name: "duplicate after a valid port", values: []string{"1", "2", "2"}, wantErr: "host port 2 is published more than once"},
		{name: "first error wins", values: []string{"80", "abc", "70000"}, wantErr: `invalid port "abc"` + syntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePorts(tt.values)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("ParsePorts(%q) error = %v, want %q", tt.values, err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("ParsePorts(%q) = %+v with an error, want nil", tt.values, got)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParsePorts(%q) = %+v, %v; want %+v", tt.values, got, err, tt.want)
			}
		})
	}
}
