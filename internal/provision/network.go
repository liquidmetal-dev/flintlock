package provision

import (
	"regexp"
	"strings"
)

var privateIPv4 = regexp.MustCompile(`^(192\.168|10\.|172\.1[6789]\.|172\.2[0-9]\.|172\.3[01]\.)`)

// LookupInterface returns the interface of the default route, replacing the
// script's "ip route show | awk '/default/ {print $5}'".
func LookupInterface(runner *Runner) (string, error) {
	out, err := runner.Output("ip", "route", "show")
	if err != nil {
		return "", err
	}

	return ParseDefaultInterface(out), nil
}

// ParseDefaultInterface extracts the default route's interface name from
// the output of "ip route show".
func ParseDefaultInterface(ipRouteShowOutput string) string {
	for _, line := range strings.Split(ipRouteShowOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "default" {
			continue
		}

		return fields[4]
	}

	return ""
}

// LookupAddress returns the private IPv4 address of the host associated
// with the given interface, replacing the script's awk/grep pipeline over
// "ip route show".
func LookupAddress(runner *Runner, iface string) (string, error) {
	out, err := runner.Output("ip", "route", "show")
	if err != nil {
		return "", err
	}

	return ParseAddressForInterface(out, iface), nil
}

// ParseAddressForInterface extracts the private IPv4 "src" address of the
// route belonging to iface from the output of "ip route show".
func ParseAddressForInterface(ipRouteShowOutput, iface string) string {
	for _, line := range strings.Split(ipRouteShowOutput, "\n") {
		if !strings.Contains(line, iface) {
			continue
		}

		fields := strings.Fields(line)
		for i, field := range fields {
			if field != "src" || i+1 >= len(fields) {
				continue
			}

			if addr := fields[i+1]; privateIPv4.MatchString(addr) {
				return addr
			}
		}
	}

	return ""
}
