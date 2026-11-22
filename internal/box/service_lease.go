package box

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const (
	serviceLeaseFileName = "service-lease"
	stoodDownFileName    = "stood-down"
	checksPerLease       = 3
	retriesPerLease      = 9
	shortestLeaseWait    = time.Second
)

type TakeOverPolicy struct {
	IsAutomatic  bool `json:"isAutomatic"`
	LapseSeconds int  `json:"lapseSeconds"`
}

type serviceLease struct {
	ConfirmedAt  time.Time `json:"confirmedAt"`
	LapseSeconds int       `json:"lapseSeconds"`
}

func leaseLength(lapseSeconds int) time.Duration {
	return time.Duration(lapseSeconds) * time.Second / 2
}

func (lease serviceLease) runsOutAt() time.Time {
	return lease.ConfirmedAt.Add(leaseLength(lease.LapseSeconds))
}

func (lease serviceLease) retryInterval() time.Duration {
	return leaseLength(lease.LapseSeconds) / retriesPerLease
}

func untilTheTakeOverNeedsACheck(session Session) time.Duration {
	if session.Role == RoleStandby && session.ServiceLapsesInSeconds != nil {
		return time.Duration(max(*session.ServiceLapsesInSeconds, 0)+1) * time.Second
	}
	if session.Role == RoleService && session.TakeOver.IsAutomatic && session.TakeOver.LapseSeconds > 0 {
		return leaseLength(session.TakeOver.LapseSeconds) / checksPerLease
	}
	return companyCheckInterval
}

func (daemon Daemon) keepServiceLease(policy TakeOverPolicy) error {
	if !policy.IsAutomatic {
		return removeIfPresent(daemon.statePath(serviceLeaseFileName))
	}
	document, errorValue := json.Marshal(serviceLease{ConfirmedAt: daemon.now(), LapseSeconds: policy.LapseSeconds})
	if errorValue != nil {
		return errorValue
	}
	return durablefile.Write(daemon.statePath(serviceLeaseFileName), document, 0o600)
}

func (daemon Daemon) runningLease() (serviceLease, bool) {
	if daemon.hasStoodDown() {
		return serviceLease{}, false
	}
	return daemon.serviceLease()
}

func (daemon Daemon) keepTheLeaseThroughTheTakeOver(policy TakeOverPolicy) error {
	if errorValue := removeIfPresent(daemon.statePath(stoodDownFileName)); errorValue != nil {
		return errorValue
	}
	return daemon.keepServiceLease(policy)
}

func (daemon Daemon) waitWithinTheLease(wait time.Duration) time.Duration {
	lease, isRunning := daemon.runningLease()
	if !isRunning {
		return wait
	}
	return min(wait, max(lease.runsOutAt().Sub(daemon.now()), shortestLeaseWait))
}

func (daemon Daemon) retryWait() time.Duration {
	lease, isRunning := daemon.runningLease()
	if !isRunning {
		return announceInterval
	}
	return min(announceInterval, max(lease.retryInterval(), shortestLeaseWait))
}

func (daemon Daemon) withinTheLease(ctx context.Context) (context.Context, context.CancelFunc) {
	lease, isRunning := daemon.runningLease()
	left := lease.runsOutAt().Sub(daemon.now())
	if !isRunning || left <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, left)
}

func (daemon Daemon) standDownWhenTheLeaseRunsOut() {
	lease, isRunning := daemon.runningLease()
	if !isRunning || daemon.StopServices == nil || daemon.now().Before(lease.runsOutAt()) {
		return
	}
	log.Printf("the central plane has not confirmed this box serves its company since %s, half the take-over lapse, so it stops its services before a standby box takes over", lease.ConfirmedAt.UTC().Format(time.RFC3339))
	daemon.forgetTheQueueAddress()
	if errorValue := daemon.StopServices(); errorValue != nil {
		log.Printf("stopping the company's services: %v; trying again at the next check", errorValue)
		return
	}
	if errorValue := durablefile.Write(daemon.statePath(stoodDownFileName), []byte(lease.ConfirmedAt.UTC().Format(time.RFC3339)+"\n"), 0o600); errorValue != nil {
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
