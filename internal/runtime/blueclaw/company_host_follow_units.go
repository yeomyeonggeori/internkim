package blueclaw

import "strings"

const (
	CompanyHostWALFollowServiceName  = "internkim-wal-follow"
	CompanyHostFileFollowServiceName = "internkim-file-follow"
	CompanyHostFileJournalSocketPath = "/run/" + CompanyHostFileFollowServiceName + "/barrier.sock"
	companyHostWALFollowVerb         = "wal-follow"
	companyHostFileFollowVerb        = "file-follow"
	companyHostFollowRestartWait     = "5s"
)

func companyHostFollowUnits(layout CompanyHostLayout) []CompanyPackageUnit {
	return []CompanyPackageUnit{
		{Name: CompanyHostWALFollowServiceName, Contents: companyHostFollowUnit(layout, "keeps every commit with the company's other boxes before it counts", companyHostWALFollowVerb, "")},
		{Name: CompanyHostFileFollowServiceName, Contents: companyHostFollowUnit(layout, "keeps every changed file with the company's other boxes", companyHostFileFollowVerb, CompanyHostFileFollowServiceName)},
	}
}

func companyHostFollowUnit(layout CompanyHostLayout, description string, verb string, runtimeDirectory string) string {
	service := []string{
		"[Service]",
		"Type=simple",
		"User=root",
		"Environment=PATH=" + layout.SearchPath(),
		"ExecStart=" + layout.BinaryPath(CompanyPackageName) + " " + verb,
		"Restart=always",
		"RestartSec=" + companyHostFollowRestartWait,
	}
	if runtimeDirectory != "" {
		service = append(service, "RuntimeDirectory="+runtimeDirectory, "RuntimeDirectoryMode=0755")
	}
	return strings.Join(append(append([]string{
		"[Unit]",
		"Description=internkim " + description,
		"Documentation=" + companyHostSnapshotDocumentation,
		"After=" + companyHostDatabaseUnitName + " network-online.target",
		"Wants=network-online.target",
		"ConditionPathExists=" + CompanyHostAgentKeyPath,
		"ConditionPathExists=" + CompanyHostBoxStatePath + "/identity.json",
		"",
	}, service...), "", "[Install]", "WantedBy=multi-user.target", ""), "\n")
}
