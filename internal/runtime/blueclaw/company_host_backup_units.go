package blueclaw

import "strings"

type CompanyPackageTimedUnits struct {
	Service CompanyPackageUnit
	Timer   CompanyPackageTimer
}

type CompanyPackageTimer struct {
	Name     string
	Contents string
}

func (timer CompanyPackageTimer) FileName() string {
	return timer.Name + ".timer"
}

func (timer CompanyPackageTimer) InstalledPath() string {
	return CompanyPackageUnitRoot + "/" + timer.FileName()
}

func CompanyPackageBackupUnits() CompanyPackageTimedUnits {
	layout := LinuxCompanyHostLayout()
	return CompanyPackageTimedUnits{
		Service: CompanyPackageUnit{Name: CompanyHostBackupServiceName, Contents: companyHostBackupService(layout)},
		Timer:   CompanyPackageTimer{Name: CompanyHostBackupServiceName, Contents: companyHostBackupTimer()},
	}
}

func companyHostBackupService(layout CompanyHostLayout) string {
	return strings.Join([]string{
		"[Unit]",
		"Description=internkim scheduled backup",
		"Documentation=" + companyHostBackupDocumentation,
		"After=" + companyHostDatabaseUnitName,
		"ConditionPathExists=" + CompanyHostAgentKeyPath,
		"",
		"[Service]",
		"Type=oneshot",
		"User=root",
		"Environment=PATH=" + layout.SearchPath(),
		"Environment=" + CompanyHostBackupsKeptSetting + "=" + CompanyHostBackupsKeptByDefault,
		"EnvironmentFile=-" + CompanyHostSettingsPath,
		"ExecStart=" + layout.BinaryPath(CompanyPackageName) + " backup --keep ${" + CompanyHostBackupsKeptSetting + "}",
		"Nice=10",
		"IOSchedulingClass=idle",
		"",
	}, "\n")
}

func companyHostBackupTimer() string {
	return strings.Join([]string{
		"[Unit]",
		"Description=Back up the internkim company host every day",
		"Documentation=" + companyHostBackupDocumentation,
		"",
		"[Timer]",
		"OnCalendar=daily",
		"RandomizedDelaySec=" + companyHostBackupRandomDelay,
		"Persistent=true",
		"",
		"[Install]",
		"WantedBy=timers.target",
		"",
	}, "\n")
}
