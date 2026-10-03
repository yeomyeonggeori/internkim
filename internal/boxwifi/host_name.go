package boxwifi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	defaultHostsPath    = "/etc/hosts"
	loopbackHostAddress = "127.0.1.1"
	hostsFileMode       = 0o644
	multicastDNSService = "avahi-daemon.service"
)

func HostNameFor(boxPublicKey string) string {
	name := strings.ToLower(SetupNetworkName + "-" + boxNameSuffix(boxPublicKey))
	name = strings.ReplaceAll(name, "_", "-")
	return strings.TrimRight(name, "-")
}

type HostNamer struct {
	Run             func(ctx context.Context, name string, arguments ...string) ([]byte, error)
	CurrentHostName func() (string, error)
	HostsPath       string
}

func (namer HostNamer) Name(ctx context.Context, boxPublicKey string) error {
	name := HostNameFor(boxPublicKey)
	current, errorValue := namer.currentHostName()
	if errorValue != nil {
		return fmt.Errorf("reading this computer's host name: %w", errorValue)
	}
	if current == name {
		return namer.pointLoopbackAt(name)
	}
	if _, errorValue := namer.run(ctx, "hostnamectl", "set-hostname", name); errorValue != nil {
		return fmt.Errorf("naming this computer %s: %w", name, errorValue)
	}
	if errorValue := namer.pointLoopbackAt(name); errorValue != nil {
		return errorValue
	}
	if _, errorValue := namer.run(ctx, "systemctl", "try-restart", multicastDNSService); errorValue != nil {
		return fmt.Errorf("restarting %s so %s.local answers: %w", multicastDNSService, name, errorValue)
	}
	return nil
}

func (namer HostNamer) run(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	if namer.Run != nil {
		return namer.Run(ctx, name, arguments...)
	}
	output, errorValue := exec.CommandContext(ctx, name, arguments...).CombinedOutput()
	if errorValue != nil {
		return output, fmt.Errorf("%w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (namer HostNamer) currentHostName() (string, error) {
	if namer.CurrentHostName != nil {
		return namer.CurrentHostName()
	}
	return os.Hostname()
}

func (namer HostNamer) pointLoopbackAt(name string) error {
	path := namer.HostsPath
	if path == "" {
		path = defaultHostsPath
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return fmt.Errorf("reading %s: %w", path, errorValue)
	}
	updated := hostsWithLoopbackName(string(document), name)
	if updated == string(document) {
		return nil
	}
	if errorValue := os.WriteFile(path, []byte(updated), hostsFileMode); errorValue != nil {
		return fmt.Errorf("pointing %s at %s in %s: %w", loopbackHostAddress, name, path, errorValue)
	}
	return nil
}

func hostsWithLoopbackName(document, name string) string {
	loopbackLine := loopbackHostAddress + "\t" + name
	lines := strings.Split(strings.TrimSuffix(document, "\n"), "\n")
	isReplaced := false
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != loopbackHostAddress {
			continue
		}
		lines[index] = loopbackLine
		isReplaced = true
	}
	if !isReplaced {
		lines = append(lines, loopbackLine)
	}
	return strings.Join(lines, "\n") + "\n"
}
