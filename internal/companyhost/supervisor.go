package companyhost

import (
	"fmt"
	"io"
	"slices"

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
	if errorValue := disableAndStop(platform, machine, platform.CompanyServiceNames(), progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be stopped on this box: %w", errorValue)
	}
	return nil
}

func disableAndStop(platform companyHostPlatform, machine Machine, names []string, progress io.Writer) error {
	if errorValue := platform.DisableServices(machine, names); errorValue != nil {
		return errorValue
	}
	return platform.StopServices(machine, names, progress)
}

func StopForTheMove(machine Machine, progress io.Writer, keepWhatWasWritten func() error) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return stopForTheMove(platform, machine, progress, keepWhatWasWritten)
}

func stopForTheMove(platform companyHostPlatform, machine Machine, progress io.Writer, keepWhatWasWritten func() error) error {
	writers, followers := writersAndFollowers(platform.CompanyServiceNames())
	behind, messenger := apartFromTheMessenger(writers)
	if errorValue := disableAndStop(platform, machine, behind, progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be stopped for the move: %w", errorValue)
	}
	if errorValue := disableAndStop(platform, machine, messenger, progress); errorValue != nil {
		return fmt.Errorf("the company's messenger could not be stopped for the move: %w", errorValue)
	}
	if errorValue := keepWhatWasWritten(); errorValue != nil {
		return errorValue
	}
	if errorValue := disableAndStop(platform, machine, dataServiceNames(), progress); errorValue != nil {
		return fmt.Errorf("the company's database could not be stopped for the move, so it could still take commits the box moved to never sees: %w", errorValue)
	}
	if errorValue := disableAndStop(platform, machine, followers, progress); errorValue != nil {
		return fmt.Errorf("the followers that keep the company's logs in the store could not be stopped for the move: %w", errorValue)
	}
	return nil
}

var followerServiceNames = []string{blueclaw.CompanyHostWALFollowServiceName, blueclaw.CompanyHostFileFollowServiceName}

var messengerServiceNames = []string{
	blueclaw.RelayServiceName,
	blueclaw.BuzzRelayServiceName,
	blueclaw.BuzzMediaServiceName,
	blueclaw.BuzzMediaGateServiceName,
}

func writersAndFollowers(names []string) ([]string, []string) {
	return partitionBy(names, followerServiceNames)
}

func apartFromTheMessenger(names []string) ([]string, []string) {
	return partitionBy(names, messengerServiceNames)
}

func partitionBy(names []string, set []string) ([]string, []string) {
	outside, inside := []string{}, []string{}
	for _, name := range names {
		if slices.Contains(set, name) {
			inside = append(inside, name)
		} else {
			outside = append(outside, name)
		}
	}
	return outside, inside
}

func StandUp(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	names := append(dataServiceNames(), platform.CompanyServiceNames()...)
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
