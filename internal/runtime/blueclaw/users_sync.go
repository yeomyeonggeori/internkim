package blueclaw

const (
	InternKimUsersSyncScriptPath      = "/usr/local/bin/internkim-users-sync"
	InternKimUsersSyncServicePath     = "/etc/systemd/system/internkim-users-sync.service"
	InternKimUsersSyncTimerPath       = "/etc/systemd/system/internkim-users-sync.timer"
	InternKimUsersSyncStatePath       = "/root/.internkim/state/users-sync.json"
	InternKimCentralPlaneAgentKeyPath = "/root/.internkim/secrets/central-plane-agent-key"
)
