package box

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	serviceLeaseFileName = "service-lease"
	stoodDownFileName    = "stood-down"
)

type TakeOverPolicy struct {
	IsAutomatic  bool `json:"isAutomatic"`
	LapseSeconds int  `json:"lapseSeconds"`
}

type serviceLease struct {
	ConfirmedAt  time.Time `json:"confirmedAt"`
	LapseSeconds int       `json:"lapseSeconds"`
}

func (lease serviceLease) hasRunOut(now time.Time) bool {
	return now.Sub(lease.ConfirmedAt) >= time.Duration(lease.LapseSeconds)*time.Second/2
}

func (daemon Daemon) keepServiceLease(policy TakeOverPolicy) error {
	if !policy.IsAutomatic {
		return removeIfPresent(daemon.statePath(serviceLeaseFileName))
	}
	document, errorValue := json.Marshal(serviceLease{ConfirmedAt: daemon.now(), LapseSeconds: policy.LapseSeconds})
	if errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(daemon.statePath(serviceLeaseFileName), document, 0o600)
}

func (daemon Daemon) standDownWhenTheLeaseRunsOut() {
	if daemon.installedCompany() == "" || daemon.StandDown == nil || daemon.hasStoodDown() {
		return
	}
	lease, isKept := daemon.serviceLease()
	if !isKept || !lease.hasRunOut(daemon.now()) {
		return
	}
	log.Printf("the central plane has not confirmed this box serves its company since %s, half the take-over lapse, so it stops its services before a standby box takes over", lease.ConfirmedAt.UTC().Format(time.RFC3339))
	if errorValue := daemon.StandDown(); errorValue != nil {
		log.Printf("stopping the company's services: %v; trying again at the next check", errorValue)
		return
	}
	if errorValue := writeFileAtomically(daemon.statePath(stoodDownFileName), []byte(lease.ConfirmedAt.UTC().Format(time.RFC3339)+"\n"), 0o600); errorValue != nil {
		log.Printf("noting that this box stood down: %v", errorValue)
	}
}

func (daemon Daemon) standUpIfItStoodDown() error {
	if !daemon.hasStoodDown() {
		return nil
	}
	log.Printf("the central plane confirms this box still serves its company, so it starts the services it stopped")
	if daemon.StandUp != nil {
		if errorValue := daemon.StandUp(); errorValue != nil {
			return errorValue
		}
	}
	return removeIfPresent(daemon.statePath(stoodDownFileName))
}

func (daemon Daemon) forgetServiceLease() error {
	return errors.Join(
		removeIfPresent(daemon.statePath(serviceLeaseFileName)),
		removeIfPresent(daemon.statePath(stoodDownFileName)),
	)
}

func (daemon Daemon) serviceLease() (serviceLease, bool) {
	document, errorValue := os.ReadFile(daemon.statePath(serviceLeaseFileName))
	if errorValue != nil {
		return serviceLease{}, false
	}
	var lease serviceLease
	if json.Unmarshal(document, &lease) != nil || lease.LapseSeconds <= 0 {
		return serviceLease{}, false
	}
	return lease, true
}

func (daemon Daemon) hasStoodDown() bool {
	_, errorValue := os.Stat(daemon.statePath(stoodDownFileName))
	return errorValue == nil
}

func (daemon Daemon) statePath(name string) string {
	return filepath.Join(daemon.Places.StateDirectoryPath, name)
}

func removeIfPresent(path string) error {
	errorValue := os.Remove(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	return errorValue
}
