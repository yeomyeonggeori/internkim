package blueclaw

import "strings"

const (
	CompanyHostWALFollowServiceName = "internkim-wal-follow"
	companyHostWALFollowVerb        = "wal-follow"
	companyHostWALFollowRestartWait = "5s"
)

func companyHostWALFollowUnit(layout CompanyHostLayout) CompanyPackageUnit {
	return CompanyPackageUnit{Name: CompanyHostWALFollowServiceName, Contents: strings.Join([]string{
		"[Unit]",
		"Description=internkim keeps every commit with the company's other boxes before it counts",
		"Documentation=" + companyHostSnapshotDocumentation,
		"After=" + companyHostDatabaseUnitName + " network-online.target",
		"Wants=network-online.target",
		"ConditionPathExists=" + CompanyHostAgentKeyPath,
		"ConditionPathExists=" + CompanyHostBoxStatePath + "/identity.json",
		"",
		"[Service]",
		"Type=simple",
		"User=root",
		"Environment=PATH=" + layout.SearchPath(),
		"ExecStart=" + layout.BinaryPath(CompanyPackageName) + " " + companyHostWALFollowVerb,
		"Restart=always",
		"RestartSec=" + companyHostWALFollowRestartWait,
		"",
		"[Install]",
		"WantedBy=multi-user.target",
		"",
	}, "\n")}
}
