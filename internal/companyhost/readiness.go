package companyhost

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// `docker compose up --wait` blocked until every health check passed, and
// systemd has no equivalent: `systemctl start` returns when the unit is active,
// and a process that answers HTTP while refusing all work is active too. This is
// that wait, against the same endpoints the compose health checks polled.
//
// It is also what decides what the person is told. "The server did not start"
// is useless; a PostgreSQL that is not accepting connections has a different
// next step from a messenger that is not ready, and each probe below carries
// its own.

// The budget the compose stack was given, kept so a slow first boot that used to
// succeed still does.
var waitForTheServerBudget = 240 * time.Second

const waitBetweenAttempts = 2 * time.Second

type serviceProbe struct {
	Service string
	// SupervisedName is this bundle's own name for the service, which each
	// platform turns into whatever its supervisor calls it.
	SupervisedName string
	Command        []string
	Answer         string
	WhenSilent     string
	WhatItCosts    string
}

func companyHostProbes(layout blueclaw.CompanyHostLayout) []serviceProbe {
	return []serviceProbe{
		databaseProbe(layout),
		cacheProbe(layout),
		{
			Service:        "the attachment store",
			SupervisedName: blueclaw.BuzzMediaServiceName,
			Command:        curlCommand("http://" + blueclaw.BuzzMediaAddress + blueclaw.BuzzMediaHealthPath),
			WhenSilent:     "the attachment store is not answering on " + blueclaw.BuzzMediaAddress,
			WhatItCosts:    "every picture and file in the messenger is read and written through it",
		},
		{
			Service:        "the messenger",
			SupervisedName: blueclaw.BuzzRelayServiceName,
			Command:        curlCommand(blueclaw.BuzzRelayReadinessURL()),
			WhenSilent:     "the messenger is not ready at " + blueclaw.BuzzRelayReadinessURL(),
			WhatItCosts:    "it is what people sign in to, and it stays up even when the agent does not",
		},
		{
			Service:        "the agent",
			SupervisedName: blueclaw.BlueclawServiceName,
			Command:        curlCommand(blueclaw.BlueclawHealthCheckURL()),
			WhenSilent:     "the agent is not answering on " + blueclaw.BlueclawBaseURL,
			WhatItCosts:    "it is what turns a message into work",
		},
		{
			Service:        "the admin gateway",
			SupervisedName: blueclaw.AdmindServiceName,
			Command:        curlCommand("http://" + blueclaw.CompanyHostAdmindListenAddress + blueclaw.BlueclawHealthCheckPath),
			WhenSilent:     "the admin gateway is not answering on " + blueclaw.CompanyHostAdmindListenAddress,
			WhatItCosts:    "the company's roster, its files and its tasks are all read through it",
		},
		{
			Service:        "the messenger bridge",
			SupervisedName: blueclaw.ChatdServiceName,
			Command:        curlCommand(blueclaw.CompanyHostChatdEndpoint + blueclaw.ChatdHealthPath),
			WhenSilent:     "the messenger bridge is not ready at " + blueclaw.CompanyHostChatdEndpoint + blueclaw.ChatdHealthPath,
			WhatItCosts:    "it hands every message people send to the agent and carries the agent's answers back",
		},
	}
}

func databaseProbe(layout blueclaw.CompanyHostLayout) serviceProbe {
	probe := serviceProbe{
		Service:        "PostgreSQL",
		SupervisedName: databaseServiceName,
		WhatItCosts:    "everything this company remembers is in it, and nothing else starts until it answers",
	}
	if layout.OwnsItsDataServices() {
		probe.Command = []string{"pg_isready", "--quiet", "--host", layout.DatabaseSocketDirectory}
		probe.WhenSilent = "PostgreSQL is not accepting connections on " + layout.DatabaseSocketDirectory
		return probe
	}
	probe.Command = []string{"pg_isready", "--quiet", "--host", "127.0.0.1", "--port", "5432"}
	probe.WhenSilent = "PostgreSQL is not accepting connections on " + layout.DatabaseLoopbackAddress
	return probe
}

func cacheProbe(layout blueclaw.CompanyHostLayout) serviceProbe {
	probe := serviceProbe{
		Service:        "the cache",
		SupervisedName: cacheServiceName,
		Answer:         "PONG",
		WhatItCosts:    "the messenger opens it for presence and fan-out and will not start without it",
	}
	if layout.OwnsItsDataServices() {
		probe.Command = []string{layout.DataServicePath(), "cache-ping"}
		probe.WhenSilent = "the cache is not answering on " + layout.CacheSocketPath
		return probe
	}
	probe.Command = []string{"redis-cli", "-h", "127.0.0.1", "ping"}
	probe.WhenSilent = "Redis is not answering on 127.0.0.1:6379"
	return probe
}

func curlCommand(address string) []string {
	return []string{"curl", "--fail", "--silent", "--show-error", "--max-time", "5", address}
}

func waitUntilTheServerAnswers(platform companyHostPlatform, machine Machine, progress io.Writer) error {
	deadline := time.Now().Add(waitForTheServerBudget)
	for _, probe := range companyHostProbes(platform.Layout()) {
		if errorValue := waitForOne(platform, machine, probe, deadline, progress); errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(progress, "  %s is ready\n", probe.Service)
	}
	return nil
}

func waitForOne(platform companyHostPlatform, machine Machine, probe serviceProbe, deadline time.Time, progress io.Writer) error {
	var lastFailure error
	for {
		answer, errorValue := machine.Output(probe.Command[0], probe.Command[1:])
		if errorValue == nil && (probe.Answer == "" || strings.Contains(answer, probe.Answer)) {
			return nil
		}
		lastFailure = errorValue
		if !time.Now().Add(waitBetweenAttempts).Before(deadline) {
			return fmt.Errorf("%s", probe.refusal(platform, machine, lastFailure))
		}
		time.Sleep(waitBetweenAttempts)
	}
}

// The refusal says which service is silent, what that costs, and the two
// commands whose output explains it. Nothing about the company was removed, and
// saying so is what stops a person from reinstalling over a working directory.
func (probe serviceProbe) refusal(platform companyHostPlatform, machine Machine, lastFailure error) string {
	identity := platform.SupervisorIdentityFor(probe.SupervisedName)
	lines := []string{
		probe.WhenSilent + ", and " + probe.WhatItCosts + ".",
		"  it was asked with: " + strings.Join(probe.Command, " "),
	}
	if lastFailure != nil {
		lines = append(lines, "  it answered: "+strings.TrimSpace(lastFailure.Error()))
	}
	if state := platform.SupervisorStateOf(machine, identity); state != "" {
		lines = append(lines, "  "+identity+" is "+state)
	}
	lines = append(lines, platform.HowToSeeWhyItIsSilent(identity)...)
	return strings.Join(append(lines,
		"Nothing was removed. This company's keys and settings are still where they were."), "\n")
}
