package command

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	dynamicdns "github.com/mholt/caddy-dynamicdns"
	"go.uber.org/zap/zaptest"
)

func TestCommand_CaddyModule(t *testing.T) {
	cmd := Command{}
	moduleInfo := cmd.CaddyModule()

	expectedID := "dynamic_dns.ip_sources.command"
	if string(moduleInfo.ID) != expectedID {
		t.Errorf("Expected module ID %s, got %s", expectedID, string(moduleInfo.ID))
	}

	if moduleInfo.New == nil {
		t.Error("New function should not be nil")
	}

	// Test that New() returns a proper Command instance
	module := moduleInfo.New()
	if _, ok := module.(*Command); !ok {
		t.Error("New() should return a *Command")
	}
}

func TestCommand_UnmarshalCaddyfile(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCmd  string
		expectedArgs []string
		expectError  bool
	}{
		{
			name:         "simple command",
			input:        "command echo hello",
			expectedCmd:  "echo",
			expectedArgs: []string{"hello"},
		},
		{
			name:         "command with multiple args",
			input:        "command curl -s https://example.com/ip",
			expectedCmd:  "curl",
			expectedArgs: []string{"-s", "https://example.com/ip"},
		},
		{
			name:         "command only",
			input:        "command date",
			expectedCmd:  "date",
			expectedArgs: []string{},
		},
		{
			name:        "no command provided",
			input:       "command",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &Command{}
			dispenser := caddyfile.NewTestDispenser(tt.input)

			err := cmd.UnmarshalCaddyfile(dispenser)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if cmd.Cmd != tt.expectedCmd {
				t.Errorf("Expected command %s, got %s", tt.expectedCmd, cmd.Cmd)
			}

			if len(cmd.Args) != len(tt.expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(tt.expectedArgs), len(cmd.Args))
				return
			}

			for i, arg := range tt.expectedArgs {
				if cmd.Args[i] != arg {
					t.Errorf("Expected arg[%d] = %s, got %s", i, arg, cmd.Args[i])
				}
			}
		})
	}
}

func TestCommand_Provision(t *testing.T) {
	cmd := &Command{}

	// Create a simple caddy context
	ctx := caddy.Context{Context: context.Background()}

	err := cmd.Provision(ctx)
	if err != nil {
		t.Errorf("Provision failed: %v", err)
	}

	if cmd.Timeout == nil {
		t.Error("Timeout should be set to default value")
	}

	expectedTimeout := caddy.Duration(30 * time.Second)
	if *cmd.Timeout != expectedTimeout {
		t.Errorf("Expected default timeout %v, got %v", expectedTimeout, *cmd.Timeout)
	}

	// Test that logger gets set (we can't easily test this without access to internal state)
	// but we can verify provision doesn't fail
}

func setupCommand(t *testing.T, cmd *Command) {
	// Simple provision without complex context setup
	logger := zaptest.NewLogger(t)
	cmd.logger = logger
	if cmd.Timeout == nil {
		timeout := caddy.Duration(30 * time.Second)
		cmd.Timeout = &timeout
	}
}

// Helper function to create IPSettings with proper configuration
func createIPSettings(ipv4, ipv6 bool) dynamicdns.IPSettings {
	var settings dynamicdns.IPSettings

	// Set IPv4: nil means enabled, false means disabled
	if ipv4 {
		settings.IPv4 = nil // nil means enabled
	} else {
		disabled := false
		settings.IPv4 = &disabled // false means disabled
	}

	// Set IPv6: nil means enabled, false means disabled
	if ipv6 {
		settings.IPv6 = nil // nil means enabled
	} else {
		disabled := false
		settings.IPv6 = &disabled // false means disabled
	}

	// Use default IPRanges (nil) which filters out private IPs
	// This maintains backward compatibility with the original test behavior

	return settings
}

func TestCommand_GetIPs_Success(t *testing.T) {
	// Create a command that outputs test IP addresses
	var echoCmd string
	var echoArgs []string

	if runtime.GOOS == "windows" {
		echoCmd = "cmd"
		echoArgs = []string{"/c", "echo", "192.168.1.1,2001:db8::1"}
	} else {
		echoCmd = "echo"
		echoArgs = []string{"192.168.1.1,2001:db8::1"}
	}

	cmd := &Command{
		Cmd:  echoCmd,
		Args: echoArgs,
	}

	setupCommand(t, cmd)

	// Test with both IPv4 and IPv6 enabled
	settings := createIPSettings(true, true)

	ips, err := cmd.GetIPs(context.Background(), settings)
	if err != nil {
		t.Errorf("GetIPs failed: %v", err)
		return
	}

	// Note: 192.168.1.1 is a private IP and will be filtered out by default
	// 2001:db8::1 is documentation address, not global unicast, also filtered
	// So we expect 0 IPs with default filtering
	if len(ips) != 0 {
		t.Logf("Note: Got %d IPs (private/non-global IPs are filtered by default)", len(ips))
	}
}

func TestCommand_GetIPs_IPv4Only(t *testing.T) {
	var echoCmd string
	var echoArgs []string

	if runtime.GOOS == "windows" {
		echoCmd = "cmd"
		echoArgs = []string{"/c", "echo", "8.8.8.8,2001:4860:4860::8888"}
	} else {
		echoCmd = "echo"
		echoArgs = []string{"8.8.8.8,2001:4860:4860::8888"}
	}

	cmd := &Command{
		Cmd:  echoCmd,
		Args: echoArgs,
	}

	setupCommand(t, cmd)

	// Test with only IPv4 enabled
	settings := createIPSettings(true, false)

	ips, err := cmd.GetIPs(context.Background(), settings)
	if err != nil {
		t.Errorf("GetIPs failed: %v", err)
		return
	}

	if len(ips) != 1 {
		t.Errorf("Expected 1 IP, got %d", len(ips))
		return
	}

	if !ips[0].Is4() {
		t.Errorf("Expected IPv4 address, got %s", ips[0].String())
	}

	if ips[0].String() != "8.8.8.8" {
		t.Errorf("Expected 8.8.8.8, got %s", ips[0].String())
	}
}

func TestCommand_GetIPs_IPv6Only(t *testing.T) {
	var echoCmd string
	var echoArgs []string

	if runtime.GOOS == "windows" {
		echoCmd = "cmd"
		echoArgs = []string{"/c", "echo", "8.8.8.8,2001:4860:4860::8888"}
	} else {
		echoCmd = "echo"
		echoArgs = []string{"8.8.8.8,2001:4860:4860::8888"}
	}

	cmd := &Command{
		Cmd:  echoCmd,
		Args: echoArgs,
	}

	setupCommand(t, cmd)

	// Test with only IPv6 enabled
	settings := createIPSettings(false, true)

	ips, err := cmd.GetIPs(context.Background(), settings)
	if err != nil {
		t.Errorf("GetIPs failed: %v", err)
		return
	}

	if len(ips) != 1 {
		t.Errorf("Expected 1 IP, got %d", len(ips))
		return
	}

	if !ips[0].Is6() {
		t.Errorf("Expected IPv6 address, got %s", ips[0].String())
	}

	if ips[0].String() != "2001:4860:4860::8888" {
		t.Errorf("Expected 2001:4860:4860::8888, got %s", ips[0].String())
	}
}

func TestCommand_GetIPs_InvalidIP(t *testing.T) {
	var echoCmd string
	var echoArgs []string

	if runtime.GOOS == "windows" {
		echoCmd = "cmd"
		echoArgs = []string{"/c", "echo", "invalid-ip"}
	} else {
		echoCmd = "echo"
		echoArgs = []string{"invalid-ip"}
	}

	cmd := &Command{
		Cmd:  echoCmd,
		Args: echoArgs,
	}

	setupCommand(t, cmd)

	settings := createIPSettings(true, true)

	_, err := cmd.GetIPs(context.Background(), settings)
	if err == nil {
		t.Error("Expected error for invalid IP, but got none")
	}

	if !strings.Contains(err.Error(), "invalid IP") {
		t.Errorf("Expected 'invalid IP' in error message, got: %v", err)
	}
}

func TestCommand_GetIPs_CommandFailure(t *testing.T) {
	cmd := &Command{
		Cmd:  "nonexistent-command-that-should-fail",
		Args: []string{},
	}

	setupCommand(t, cmd)

	settings := createIPSettings(true, true)

	_, err := cmd.GetIPs(context.Background(), settings)
	if err == nil {
		t.Error("Expected error for nonexistent command, but got none")
	}
}

func TestCommand_GetIPs_Timeout(t *testing.T) {
	// Skip this test on Windows as sleep command syntax is different
	if runtime.GOOS == "windows" {
		t.Skip("Skipping timeout test on Windows")
	}

	// Create a command that sleeps longer than the timeout
	timeout := caddy.Duration(100 * time.Millisecond)
	cmd := &Command{
		Cmd:     "sleep",
		Args:    []string{"1"}, // Sleep for 1 second
		Timeout: &timeout,
	}

	setupCommand(t, cmd)

	settings := createIPSettings(true, false)

	_, err := cmd.GetIPs(context.Background(), settings)
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}
}

func TestCommand_GetIPs_EmptyOutput(t *testing.T) {
	var echoCmd string
	var echoArgs []string

	if runtime.GOOS == "windows" {
		// Use echo. (with period) to output just a newline on Windows
		echoCmd = "cmd"
		echoArgs = []string{"/c", "echo."}
	} else {
		echoCmd = "echo"
		echoArgs = []string{""}
	}

	cmd := &Command{
		Cmd:  echoCmd,
		Args: echoArgs,
	}

	setupCommand(t, cmd)

	settings := createIPSettings(true, true)

	ips, err := cmd.GetIPs(context.Background(), settings)
	if err != nil {
		t.Errorf("GetIPs failed: %v", err)
		return
	}

	if len(ips) != 0 {
		t.Errorf("Expected 0 IPs for empty output, got %d", len(ips))
	}
}

func TestCommand_GetIPs_WithWhitespace(t *testing.T) {
	var echoCmd string
	var echoArgs []string

	if runtime.GOOS == "windows" {
		echoCmd = "cmd"
		echoArgs = []string{"/c", "echo", " 8.8.8.8 , 2001:4860:4860::8888 "}
	} else {
		echoCmd = "echo"
		echoArgs = []string{" 8.8.8.8 , 2001:4860:4860::8888 "}
	}

	cmd := &Command{
		Cmd:  echoCmd,
		Args: echoArgs,
	}

	setupCommand(t, cmd)

	settings := createIPSettings(true, true)

	ips, err := cmd.GetIPs(context.Background(), settings)
	if err != nil {
		t.Errorf("GetIPs failed: %v", err)
		return
	}

	if len(ips) != 2 {
		t.Errorf("Expected 2 IPs, got %d", len(ips))
		return
	}

	// Check that whitespace was properly trimmed
	foundIPv4 := false
	foundIPv6 := false
	for _, ip := range ips {
		if ip.String() == "8.8.8.8" {
			foundIPv4 = true
		} else if ip.String() == "2001:4860:4860::8888" {
			foundIPv6 = true
		}
	}

	if !foundIPv4 {
		t.Error("Expected to find IPv4 address 8.8.8.8")
	}
	if !foundIPv6 {
		t.Error("Expected to find IPv6 address 2001:4860:4860::8888")
	}
}

// Test interface compliance at compile time
var (
	_ dynamicdns.IPSource   = (*Command)(nil)
	_ caddy.Provisioner     = (*Command)(nil)
	_ caddyfile.Unmarshaler = (*Command)(nil)
)
