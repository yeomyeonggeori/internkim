package companyhost

import (
	"fmt"
	"io"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type ServiceState string

const (
	ServiceRunning ServiceState = "running"
	ServiceFailed  ServiceState = "failed"
	ServiceStopped ServiceState = "stopped"
)

type serviceSupervisor interface {
	EnableServices(machine Machine, names []string) error
	DisableServices(machine Machine, names []string) error
	StartServices(machine Machine, names []string, progress io.Writer) error
	StopServices(machine Machine, names []string, progress io.Writer) error
	RestartServices(machine Machine, names []string, progress io.Writer) error
	StateOfService(machine Machine, name string) ServiceState
}

type BoxAbilities struct {
	CanSetUpWifi        bool
	CanRenameItself     bool
	CanSetAdminPassword bool
}

func BoxAbilitiesOfThisMachine() (BoxAbilities, error) {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return BoxAbilities{}, errorValue
	}
	return platform.BoxAbilities(), nil
}

func StateOfTheService(machine Machine, name string) (ServiceState, error) {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return "", errorValue
	}
	return platform.StateOfService(machine, name), nil
}

func StartTheService(machine Machine, name string, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return platform.StartServices(machine, []string{name}, progress)
}

func StopTheService(machine Machine, name string, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return platform.StopServices(machine, []string{name}, progress)
}

func TakesOverACompany() error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = backupPlatformOf(platform)
	return errorValue
}

func KeepTheBoxSessionFresh(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return platform.SuperviseTheBox(machine, progress)
}

func superviseTheBoxService(platform companyHostPlatform, machine Machine, progress io.Writer) error {
	box := []string{blueclaw.BoxServiceName}
	if errorValue := platform.EnableServices(machine, box); errorValue != nil {
		return fmt.Errorf("the service that keeps this computer's session fresh could not be enabled: %w", errorValue)
	}
	return platform.RestartServices(machine, box, progress)
}

func dataServiceNames() []string {
	return []string{blueclaw.CompanyHostDatabaseServiceName, blueclaw.CompanyHostCacheServiceName}
}

func StopTheServices(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return stopTheCompanyServices(platform, machine, progress)
}

func stopTheCompanyServices(platform companyHostPlatform, machine Machine, progress io.Writer) error {
	names := platform.CompanyServiceNames()
	if errorValue := platform.DisableServices(machine, names); errorValue != nil {
		return fmt.Errorf("the company's services could not be kept from starting with this box: %w", errorValue)
	}
	if errorValue := platform.StopServices(machine, names, progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be stopped on this box: %w", errorValue)
	}
	return nil
}

func StandUp(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	names := platform.CompanyServiceNames()
	if errorValue := platform.EnableServices(machine, names); errorValue != nil {
		return fmt.Errorf("the company's services could not be set to start with this box again: %w", errorValue)
	}
	if errorValue := platform.StartServices(machine, names, progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be started again on this box: %w", errorValue)
	}
	return nil
}

func StandDown(machine Machine, progress io.Writer) error {
	if errorValue := StopTheServices(machine, progress); errorValue != nil {
		return errorValue
	}
	return forgetTheCurrentCompany()
}

func stopTheHost(platform companyHostPlatform, machine Machine, progress io.Writer) error {
	names := append([]string{blueclaw.BoxServiceName}, platform.CompanyServiceNames()...)
	if errorValue := platform.StopServices(machine, names, progress); errorValue != nil {
		return fmt.Errorf("the host's services could not be stopped, so nothing was restored: %w", errorValue)
	}
	return nil
}
