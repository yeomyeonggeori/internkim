package blueclaw

import "strings"

const (
	CompanyHostWALFollowServiceName  = "internkim-wal-follow"
	CompanyHostFileFollowServiceName = "internkim-file-follow"
	CompanyHostFileJournalSocketPath = "/run/" + CompanyHostFileFollowServiceName + "/barrier.sock"
	companyHostFollowRestartWait     = "5s"
)

type companyHostFollower struct {
	name             string
	description      string
	verb             string
	isBoxOnly        bool
	runtimeDirectory string
}

// The WAL follower runs only on a box. The file follower runs on every Linux
// host because it also answers the agent's barrier: a box keeps the files
// before answering, any other host answers that it keeps no journal.
var companyHostFollowers = []companyHostFollower{
	{name: CompanyHostWALFollowServiceName, description: "keeps every commit with the company's other boxes before it counts", verb: "wal-follow", isBoxOnly: true},
	{name: CompanyHostFileFollowServiceName, description: "keeps every changed file with the company's other boxes", verb: "file-follow", runtimeDirectory: CompanyHostFileFollowServiceName},
}

func companyHostFollowUnits(layout CompanyHostLayout) []CompanyPackageUnit {
	units := []CompanyPackageUnit{}
	for _, follower := range companyHostFollowers {
		units = append(units, CompanyPackageUnit{Name: follower.name, Contents: follower.unit(layout)})
	}
	return units
}

func (follower companyHostFollower) unit(layout CompanyHostLayout) string {
	lines := []string{
		"[Unit]",
		"Description=internkim " + follower.description,
		"Documentation=" + companyHostSnapshotDocumentation,
		"After=" + companyHostDatabaseUnitName + " network-online.target",
		"Wants=network-online.target",
		"ConditionPathExists=" + CompanyHostAgentKeyPath,
	}
	if follower.isBoxOnly {
		lines = append(lines, "ConditionPathExists="+CompanyHostBoxStatePath+"/identity.json")
	}
	lines = append(lines,
		"",
		"[Service]",
		"Type=simple",
		"User=root",
		"Environment=PATH="+layout.SearchPath(),
		"ExecStart="+layout.BinaryPath(CompanyPackageName)+" "+follower.verb,
		"Restart=always",
		"RestartSec="+companyHostFollowRestartWait,
	)
	if follower.runtimeDirectory != "" {
		lines = append(lines, "RuntimeDirectory="+follower.runtimeDirectory, "RuntimeDirectoryMode=0755")
	}
	return strings.Join(append(lines, "", "[Install]", "WantedBy=multi-user.target", ""), "\n")
}
