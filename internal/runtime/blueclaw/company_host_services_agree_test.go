package blueclaw

import (
	"slices"
	"strings"
	"testing"
)

func startedWith(t *testing.T, serviceName string, flag string) string {
	t.Helper()
	service, isBundled := CompanyHostServiceNamed(LinuxCompanyHostLayout(), serviceName)
	if !isBundled {
		t.Fatalf("the bundle carries no %s", serviceName)
	}
	for index := 1; index+1 < len(service.Command); index++ {
		if service.Command[index] == flag {
			return service.Command[index+1]
		}
	}
	t.Fatalf("the package starts %s without %s", serviceName, flag)
	return ""
}

func TestCapabilitydAndAdmindAreToldTheSameMessenger(t *testing.T) {
	if startedWith(t, AdmindServiceName, "-chatd-platform") != startedWith(t, CapabilitydServiceName, "--chatd-platform") {
		t.Fatal("the package names a different messenger to each daemon, so a message leaves on whichever one capabilityd was told and answers \"sent\" either way")
	}
}

func TestAdmindAndBlueclawShareOnePolicyFile(t *testing.T) {
	if startedWith(t, AdmindServiceName, "-blueclaw-policy") != startedWith(t, BlueclawServiceName, "-policy") {
		t.Fatal("admind reconciles the company roster onto one file and blueclaw reads another, so nobody the company hires reaches the agent")
	}
}

func TestThePolicyAdmindRewritesIsWritable(t *testing.T) {
	if strings.HasPrefix(startedWith(t, AdmindServiceName, "-blueclaw-policy"), "/etc/") {
		t.Fatal("admind replaces the policy by writing a temp beside it and renaming, so a path under a read-only /etc fails every reconcile")
	}
}

func TestAdmindIsToldWhereTheBuzzSeedLives(t *testing.T) {
	if startedWith(t, AdmindServiceName, "-buzz-key-seed-path") == "" {
		t.Fatal("without the seed admind cannot sign a message under a person's own name, and answers as though it had")
	}
}

func TestBlueclawStartsWithTheSkillsThePackageShips(t *testing.T) {
	service, _ := CompanyHostServiceNamed(LinuxCompanyHostLayout(), BlueclawServiceName)
	skillsPath, isSet := environmentSettingOf(service, "BLUECLAW_BUNDLED_SKILLS_PATH")
	if !isSet || skillsPath != CompanyPackageSkillsPath {
		t.Fatalf("blueclaw looks for its skills at %q and the package puts them at %s, so it starts with nothing", skillsPath, CompanyPackageSkillsPath)
	}
}

func TestTheCompanyHostAdmindInstallsNoDeviceUsersSync(t *testing.T) {
	service, _ := CompanyHostServiceNamed(LinuxCompanyHostLayout(), AdmindServiceName)
	if !slices.Contains(service.Command, "-install-users-sync=false") {
		t.Fatalf("the package starts admind without -install-users-sync=false, so it writes the device's users-sync timer onto the host, " +
			"where the script finds no company directory and fails every hour from units the package does not own")
	}
}

func TestTheDeviceAdmindStillInstallsItsUsersSync(t *testing.T) {
	if strings.Contains(AdmindServiceUnit(), "-install-users-sync") {
		t.Fatal("the device's admind unit turns off the users sync the device has always installed, a change the frozen device path does not take")
	}
}
