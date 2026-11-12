package blueclaw

import "strings"

const (
	CompanyHostWALFollowServiceName  = "internkim-wal-follow"
	CompanyHostFileFollowServiceName = "internkim-file-follow"
	CompanyHostFileJournalSocketPath = "/run/" + CompanyHostFileFollowServiceName + "/barrier.sock"
	companyHostFollowRestartWait     = "5s"
	CompanyHostSeveralBoxesPath      = CompanyHostBoxStatePath + "/several-boxes"
)

type companyHostFollower struct {
	name             string
	description      string
	verb             string
	runtimeDirectory string
}

var companyHostFollowers = []companyHostFollower{
	{name: CompanyHostWALFollowServiceName, description: "keeps every commit with the company's other boxes before it counts", verb: "wal-follow"},
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
		"",
		"[Service]",
		"Type=simple",
		"User=root",
		"Environment=PATH=" + layout.SearchPath(),
		"ExecStart=" + layout.BinaryPath(CompanyPackageName) + " " + follower.verb,
		"Restart=always",
		"RestartSec=" + companyHostFollowRestartWait,
	}
	if follower.runtimeDirectory != "" {
		lines = append(lines, "RuntimeDirectory="+follower.runtimeDirectory, "RuntimeDirectoryMode=0755")
	}
	return strings.Join(append(lines, "", "[Install]", "WantedBy=multi-user.target", ""), "\n")
}
