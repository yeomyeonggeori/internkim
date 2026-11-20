package boxwifi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const (
	defaultHostsPath    = "/etc/hosts"
	loopbackHostAddress = "127.0.1.1"
	hostsFileMode       = 0o644
	multicastDNSService = "avahi-daemon.service"
	givenHostNameFile   = "host-name"
)

var hostLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func HostNameFor(companySlug string) string {
	if !hostLabel.MatchString(companySlug) {
		return EmptyBoxHostName
	}
	return companySlug
}

type HostNamer struct {
	Run                func(ctx context.Context, name string, arguments ...string) ([]byte, error)
	CurrentHostName    func() (string, error)
	HostsPath          string
	StateDirectoryPath string
}

func (namer HostNamer) Name(ctx context.Context, companySlug string) error {
	current, errorValue := namer.currentHostName()
	if errorValue != nil {
		return fmt.Errorf("reading this computer's host name: %w", errorValue)
	}
	if !namer.canRename(current) {
		return nil
	}
	name := HostNameFor(companySlug)
	if current == name {
		return namer.pointLoopbackAt(name)
	}
	if _, errorValue := namer.run(ctx, "hostnamectl", "set-hostname", name); errorValue != nil {
		return fmt.Errorf("naming this computer %s: %w", name, errorValue)
	}
	if errorValue := namer.rememberGivenName(name); errorValue != nil {
		return errorValue
	}
	if errorValue := namer.pointLoopbackAt(name); errorValue != nil {
		return errorValue
	}
	if _, errorValue := namer.run(ctx, "systemctl", "try-restart", multicastDNSService); errorValue != nil {
		return fmt.Errorf("restarting %s so %s.local answers: %w", multicastDNSService, name, errorValue)
	}
	return nil
}

func (namer HostNamer) AdoptCurrentName() error {
	if namer.givenName() != "" {
		return nil
	}
	current, errorValue := namer.currentHostName()
	if errorValue != nil {
		return fmt.Errorf("reading this computer's host name: %w", errorValue)
	}
	return namer.rememberGivenName(current)
}

func (namer HostNamer) canRename(current string) bool {
	return isEmptyBoxHostName(current) || current == namer.givenName()
}

func isEmptyBoxHostName(name string) bool {
	suffix, isEmptyBoxName := strings.CutPrefix(name, EmptyBoxHostName)
	if !isEmptyBoxName || suffix == "" {
		return isEmptyBoxName
	}
	number, isNumbered := strings.CutPrefix(suffix, "-")
	return isNumbered && number != "" && strings.Trim(number, "0123456789") == ""
}

func (namer HostNamer) givenName() string {
	document, errorValue := os.ReadFile(namer.givenNamePath())
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func (namer HostNamer) rememberGivenName(name string) error {
	if errorValue := os.MkdirAll(namer.StateDirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := durablefile.Write(namer.givenNamePath(), []byte(name+"\n"), 0o600); errorValue != nil {
		return fmt.Errorf("remembering that InternKim named this computer %s: %w", name, errorValue)
	}
	return nil
}

func (namer HostNamer) givenNamePath() string {
	return filepath.Join(namer.StateDirectoryPath, givenHostNameFile)
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
