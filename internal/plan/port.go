package plan

import (
	"fmt"
	"strings"
)

// LoopbackIP is the only host address caged publishes ports on.
const LoopbackIP = "127.0.0.1"

// Port is one published container port: HostIP:HostPort on the host maps to
// ContainerPort in the container, over TCP.
type Port struct {
	HostIP                  string
	HostPort, ContainerPort int
}

// maxPort is the highest valid port number.
const maxPort = 65535

// ParsePorts parses the values of repeated -p flags. Each value is PORT or
// HOST_PORT:CONTAINER_PORT, with decimal ports in 1-65535. Every port binds
// to LoopbackIP. A host port may appear only once.
func ParsePorts(values []string) ([]Port, error) {
	var ports []Port
	published := make(map[int]struct{}, len(values))
	for _, value := range values {
		port, err := parsePort(value)
		if err != nil {
			return nil, err
		}
		if _, ok := published[port.HostPort]; ok {
			return nil, fmt.Errorf("host port %d is published more than once", port.HostPort)
		}
		published[port.HostPort] = struct{}{}
		ports = append(ports, port)
	}
	return ports, nil
}

// parsePort parses one -p value. It names the most specific problem it can:
// an unsupported form (host address, protocol, range) before bad syntax, and
// bad syntax before a number out of range.
func parsePort(value string) (Port, error) {
	parts := strings.Split(value, ":")
	if len(parts) > 2 {
		return Port{}, fmt.Errorf("invalid port %q: a host address is not supported; caged always binds %s", value, LoopbackIP)
	}
	if strings.Contains(value, "/") {
		return Port{}, fmt.Errorf("invalid port %q: protocols are not supported; caged publishes TCP only", value)
	}
	for _, part := range parts {
		if isRange(part) {
			return Port{}, fmt.Errorf("invalid port %q: port ranges are not supported", value)
		}
	}
	for _, part := range parts {
		if !allDigits(part) {
			return Port{}, fmt.Errorf("invalid port %q; use PORT or HOST_PORT:CONTAINER_PORT, e.g. -p 3000 or -p 8080:3000", value)
		}
	}
	numbers := make([]int, len(parts))
	for i, part := range parts {
		number, ok := portNumber(part)
		if !ok {
			return Port{}, fmt.Errorf("invalid port %q: %s is out of range 1-%d", value, part, maxPort)
		}
		numbers[i] = number
	}
	port := Port{HostIP: LoopbackIP, HostPort: numbers[0], ContainerPort: numbers[0]}
	if len(numbers) == 2 {
		port.ContainerPort = numbers[1]
	}
	return port, nil
}

// allDigits reports whether s is one or more ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isRange reports whether s looks like LOW-HIGH: digits, one dash, digits.
func isRange(s string) bool {
	low, high, found := strings.Cut(s, "-")
	return found && allDigits(low) && allDigits(high)
}

// portNumber converts a string of digits to a port in 1-65535. Leading zeros
// are allowed.
func portNumber(digits string) (int, bool) {
	number := 0
	for i := 0; i < len(digits); i++ {
		number = number*10 + int(digits[i]-'0')
		if number > maxPort {
			return 0, false
		}
	}
	return number, number >= 1
}
