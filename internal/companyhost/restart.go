package companyhost

import "io"

func RestartTheServices(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return platform.SuperviseTheBundle(machine, progress)
}
