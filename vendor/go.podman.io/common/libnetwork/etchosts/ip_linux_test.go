//go:build linux

package etchosts

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func TestIsWSLMirrored(t *testing.T) {
	origCmd := wslNetworkingModeCmd
	defer func() { wslNetworkingModeCmd = origCmd }()

	tests := []struct {
		name     string
		mockOut  string
		mockErr  error
		expected bool
	}{
		{
			name:     "mirrored mode trimmed",
			mockOut:  "mirrored",
			mockErr:  nil,
			expected: true,
		},
		{
			name:     "mirrored mode with whitespace and newlines",
			mockOut:  "  mirrored \r\n",
			mockErr:  nil,
			expected: true,
		},
		{
			name:     "nat mode",
			mockOut:  "nat",
			mockErr:  nil,
			expected: false,
		},
		{
			name:     "virtioproxy mode",
			mockOut:  "virtioproxy",
			mockErr:  nil,
			expected: false,
		},
		{
			name:     "command error (e.g. wslinfo not found)",
			mockOut:  "",
			mockErr:  errors.New("executable not found"),
			expected: false,
		},
		{
			name:     "empty output",
			mockOut:  "",
			mockErr:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wslNetworkingModeCmd = func() (string, error) {
				return tt.mockOut, tt.mockErr
			}
			assert.Equal(t, tt.expected, isWSLMirrored())
		})
	}
}

func TestGetMirroredHostIP(t *testing.T) {
	// With nil gateway and negative link index, it should return empty string gracefully
	ip := getMirroredHostIP(nil, -1)
	assert.Empty(t, ip)

	// If there is a default route present, testing with its gateway and link should return a valid IPv4
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err == nil {
		for _, r := range routes {
			if (r.Dst == nil || r.Dst.String() == defaultWSLRoute) && r.Gw != nil {
				resolved := getMirroredHostIP(r.Gw, r.LinkIndex)
				assert.NotEmpty(t, resolved)
				parsed := net.ParseIP(resolved)
				assert.NotNil(t, parsed)
				assert.NotNil(t, parsed.To4())
				break
			}
		}
	}
}

func TestWSLHostIP(t *testing.T) {
	// In an environment with default route, wslHostIP should return a valid IP or empty string without crashing
	ip := wslHostIP()
	if ip != "" {
		parsed := net.ParseIP(ip)
		assert.NotNil(t, parsed, "returned IP should be valid")
		assert.NotNil(t, parsed.To4(), "returned IP should be IPv4")
	}
}
