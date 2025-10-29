// Copyright (c) 2023 Nils Stein
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package command

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	dynamicdns "github.com/mholt/caddy-dynamicdns"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(Command{})
}

// Command is an IP source that looks up the public IP addresses by
// executing a script or command from your filesystem.
//
// The command must return the IP addresses comma spreaded in plain text.
type Command struct {
	// The command to execute.
	Cmd string `json:"command,omitempty"`

	// Arguments to the command. Placeholders are expanded
	// in arguments, so use caution to not introduce any
	// security vulnerabilities with the command.
	Args []string `json:"args,omitempty"`

	// The directory in which to run the command.
	Dir string `json:"dir,omitempty"`

	// How long to wait for the command to terminate
	// before forcefully closing it. Default: 30s
	Timeout *caddy.Duration `json:"timeout,omitempty"`

	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (Command) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "dynamic_dns.ip_sources.command",
		New: func() caddy.Module { return new(Command) },
	}
}

// UnmarshalCaddyfile parses the module's Caddyfile config. Syntax:
//
//	exec <command> <args...>
func (c *Command) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if !d.NextArg() {
			return d.ArgErr()
		}
		c.Cmd = d.Val()
		c.Args = d.RemainingArgs()
	}
	return nil
}

// Provision sets up the module.
func (c *Command) Provision(ctx caddy.Context) error {
	c.logger = ctx.Logger(c)
	if c.Timeout == nil || *c.Timeout <= 0 {
		timeout := caddy.Duration(30 * time.Second)
		c.Timeout = &timeout
	}
	return nil
}

// GetIPs gets the public addresses of this machine.
func (c Command) GetIPs(ctx context.Context, settings dynamicdns.IPSettings) ([]netip.Addr, error) {
	out := []netip.Addr{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var cancel context.CancelFunc

	replacer := caddy.NewReplacer()

	// expand placeholders in command args;
	// notably, we do not expand placeholders
	// in the command itself for safety reasons
	expandedArgs := make([]string, len(c.Args))
	for i := range c.Args {
		expandedArgs[i] = replacer.ReplaceAll(c.Args[i], "")
	}

	if c.Timeout != nil && *c.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*c.Timeout))
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.Cmd, expandedArgs...)
	cmd.Dir = c.Dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.logger.Debug("running command",
		zap.String("command", c.Cmd),
		zap.Strings("args", expandedArgs),
		zap.String("dir", c.Dir),
		zap.Int64("timeout", int64(time.Duration(*c.Timeout))),
	)

	err := cmd.Run()
	if err != nil {
		c.logger.Error("command execution failed",
			zap.String("command", c.Cmd),
			zap.Strings("args", expandedArgs),
			zap.String("dir", c.Dir),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
			zap.Error(err))
		return nil, err
	}

	exitCode := cmd.ProcessState.ExitCode()
	if exitCode != 0 {
		c.logger.Error("command execution failed",
			zap.String("command", c.Cmd),
			zap.Strings("args", expandedArgs),
			zap.String("dir", c.Dir),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
			zap.Int("exit_code", exitCode))
		return nil, fmt.Errorf("command %s exited with: %d", c.Cmd, exitCode)
	}

	// Log stderr as warning if present but command succeeded
	if len(stderr.String()) > 0 {
		c.logger.Warn("command produced stderr output",
			zap.String("command", c.Cmd),
			zap.String("stderr", stderr.String()))
	}

	ipArr := strings.Split(strings.TrimSpace(stdout.String()), ",")
	for i := 0; i < len(ipArr); i++ {
		ipStr := strings.TrimSpace(ipArr[i])
		if ipStr == "" {
			continue // Skip empty strings
		}

		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			c.logger.Error("parsing IP failed",
				zap.String("command", c.Cmd),
				zap.Strings("args", expandedArgs),
				zap.String("stdout", stdout.String()),
				zap.String("ip", ipStr),
				zap.Error(err))
			return nil, fmt.Errorf("invalid IP: %s", ipStr)
		}

		// Filter based on IP version requirements and IP range filtering
		if settings.V4Enabled() && addr.Is4() && settings.Contains(addr) {
			out = append(out, addr)
			c.logger.Debug("parsed IPv4 successfully",
				zap.String("command", c.Cmd),
				zap.Strings("args", expandedArgs),
				zap.String("ip", addr.String()))
		} else if settings.V6Enabled() && addr.Is6() && settings.Contains(addr) {
			out = append(out, addr)
			c.logger.Debug("parsed IPv6 successfully",
				zap.String("command", c.Cmd),
				zap.Strings("args", expandedArgs),
				zap.String("ip", addr.String()))
		}
	}

	return out, nil
}

// Interface guards
var (
	_ dynamicdns.IPSource   = (*Command)(nil)
	_ caddy.Provisioner     = (*Command)(nil)
	_ caddyfile.Unmarshaler = (*Command)(nil)
)
