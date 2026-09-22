package companyhost

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// Every unit started here is rendered by internal/runtime/blueclaw. The package
// renders them at build time and this renders them at install time, from the
// same functions, so the two paths cannot supervise the company host
// differently. A file that is already on disk belongs to whatever put it there,
// which on a packaged box is dpkg, and is left alone.

// The database and the cache are the distribution's units, not ours. Nothing we
// ship starts them, so the install does, in the order the bundle needs them.
var distributionServices = []string{"postgresql", "redis-server"}

func startServices(machine Machine, progress io.Writer) error {
	if errorValue := installServiceUnits(blueclaw.CompanyPackageUnitRoot, progress); errorValue != nil {
		return errorValue
	}
	if errorValue := machine.Run("systemctl", []string{"daemon-reload"}, nil, progress); errorValue != nil {
		return fmt.Errorf("systemd would not reload its units; this package supervises the company server with systemd: %w", errorValue)
	}
	names := companyHostUnitFileNames()
	if errorValue := machine.Run("systemctl", append([]string{"enable"}, names...), nil, io.Discard); errorValue != nil {
		return fmt.Errorf("the company server's services could not be enabled, so they would not come back after a restart: %w", errorValue)
	}
	if errorValue := machine.Run("systemctl", append([]string{"restart"}, names...), nil, progress); errorValue != nil {
		return fmt.Errorf("the company server's services could not be started: %w", errorValue)
	}
	return nil
}

func startDistributionServices(machine Machine) error {
	arguments := append([]string{"enable", "--now"}, distributionServices...)
	if errorValue := machine.Run("systemctl", arguments, nil, io.Discard); errorValue != nil {
		return fmt.Errorf(
			"%s could not be started. They are the distribution's own services and the company server keeps everything it knows in them: %w",
			strings.Join(distributionServices, " and "), errorValue)
	}
	return nil
}

// A unit already on disk belongs to whatever put it there, which on a packaged
// box is dpkg: rewriting it would make `dpkg --verify` report the package
// modified. What is missing is written from the same renderer the package built
// from, which is the whole of the claim that the two paths supervise the company
// host identically.
func installServiceUnits(unitRoot string, progress io.Writer) error {
	if errorValue := os.MkdirAll(unitRoot, 0o755); errorValue != nil {
		return errorValue
	}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		path := filepath.Join(unitRoot, unit.FileName())
		if _, errorValue := os.Stat(path); errorValue == nil {
			continue
		}
		if errorValue := os.WriteFile(path, []byte(unit.Contents), 0o644); errorValue != nil {
			return fmt.Errorf("write the %s unit: %w", unit.Name, errorValue)
		}
		fmt.Fprintf(progress, "  wrote %s\n", path)
	}
	return nil
}

func companyHostUnitFileNames() []string {
	names := []string{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		names = append(names, unit.FileName())
	}
	return names
}
