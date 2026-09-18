package etchosts

import (
	"net"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const defaultWSLRoute = "0.0.0.0/0"

// wslNetworkingModeCmd retrieves the WSL networking mode using the official wslinfo utility.
// It is defined as a variable to allow mocking in unit tests.
var wslNetworkingModeCmd = func() (string, error) {
	out, err := exec.Command("wslinfo", "--networking-mode").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// isWSLMirrored checks whether the current WSL environment is configured with networkingMode=mirrored.
func isWSLMirrored() bool {
	mode, err := wslNetworkingModeCmd()
	if err != nil {
		return false
	}
	return strings.TrimSpace(mode) == "mirrored"
}

// getMirroredHostIP returns the Windows host IP address when running in WSL mirrored mode.
// In mirrored mode, Windows network interfaces are mirrored into Linux.
// Rather than using the default route's gateway (which points to the external network router),
// we resolve the local host IP associated with the default route interface.
func getMirroredHostIP(defaultGw net.IP, linkIndex int) string {
	// Strategy 1: Ask the kernel routing table for the preferred source IP
	// used to reach the default gateway.
	if len(defaultGw) > 0 {
		routes, err := netlink.RouteGet(defaultGw)
		if err == nil && len(routes) > 0 && routes[0].Src != nil && !routes[0].Src.IsLoopback() && routes[0].Src.To4() != nil {
			return routes[0].Src.String()
		}
	}

	// Strategy 2: Fallback to the first global unicast IPv4 address assigned to the
	// interface of the default route.
	if linkIndex > 0 {
		iface, err := net.InterfaceByIndex(linkIndex)
		if err == nil {
			addrs, err := iface.Addrs()
			if err == nil {
				for _, addr := range addrs {
					if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil && ipNet.IP.IsGlobalUnicast() {
						return ipNet.IP.String()
					}
				}
			}
		}
	}

	return ""
}

// wslHostIP returns the Windows host's IP address. It only makes
// sense to execute it when running in a WSL distribution.
//
// In WSL NAT mode, instructions to retrieve the IP address are section "Identify IP address"
// (scenario 2) of the WSL networking documentation:
// https://learn.microsoft.com/en-us/windows/wsl/networking#identify-ip-address
// where the default route gateway represents the Windows host.
//
// In WSL mirrored mode, the Windows host's network interfaces are mirrored into Linux,
// and the default route gateway points to the upstream LAN router instead of Windows.
// In that mode, we determine the Windows host IP from the mirrored interface.
func wslHostIP() string {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		logrus.Warnf("Failed getting routes in the WSL machine: %v", err)
		return ""
	}
	for _, r := range routes {
		if (r.Dst == nil || r.Dst.String() == defaultWSLRoute) && r.Gw != nil {
			if isWSLMirrored() {
				if hostIP := getMirroredHostIP(r.Gw, r.LinkIndex); hostIP != "" {
					return hostIP
				}
				logrus.Warnf("Failed to determine mirrored host IP, falling back to default gateway")
			}
			return r.Gw.String()
		}
	}
	logrus.Warnf("No default route found in the WSL machine")
	return ""
}
