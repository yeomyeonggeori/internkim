package blueclaw

import (
	"strings"
	"testing"
)

func boxUnitNamed(t *testing.T, name string) CompanyPackageUnit {
	t.Helper()
	for _, unit := range CompanyHostBoxSystemdUnits(LinuxCompanyHostLayout()) {
		if unit.Name == name {
			return unit
		}
	}
	t.Fatalf("the package renders no %s unit", name)
	return CompanyPackageUnit{}
}

func boxDaemonNamed(t *testing.T, name string) CompanyHostLaunchDaemon {
	t.Helper()
	daemons, errorValue := CompanyHostBoxLaunchDaemons(MacCompanyHostLayout("/opt/homebrew"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, daemon := range daemons {
		if daemon.ServiceName == name {
			return daemon
		}
	}
	t.Fatalf("a Mac renders no %s daemon", name)
	return CompanyHostLaunchDaemon{}
}

func TestTheBoxUnitStartsWithTheComputerAndComesBackAfterACrash(t *testing.T) {
	unit := boxUnitNamed(t, BoxServiceName)
	for _, line := range []string{
		"ExecStart=" + CompanyPackageBinaryRoot + "/internkim box",
		"User=root",
		"Restart=on-failure",
		"RestartSec=30",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit.Contents, line+"\n") {
			t.Fatalf("the box unit lacks %q:\n%s", line, unit.Contents)
		}
	}
	if unit.StartsOnlyWhenAsked {
		t.Fatal("the box unit waits to be asked, so a box nobody configured never announces itself")
	}
}

func TestTheTakeOverUnitStartsOnlyWhenTheBoxAsksAndBringsTheBoxBackWhenItFails(t *testing.T) {
	unit := boxUnitNamed(t, CompanyHostTakeOverServiceName)
	for _, line := range []string{
		"ExecStart=" + CompanyPackageBinaryRoot + "/internkim restore --taking-over",
		"OnFailure=" + BoxServiceName + ".service",
		"User=root",
	} {
		if !strings.Contains(unit.Contents, line+"\n") {
			t.Fatalf("the take-over unit lacks %q:\n%s", line, unit.Contents)
		}
	}
	for _, absent := range []string{"[Install]", "Restart=", "ConditionPathExists="} {
		if strings.Contains(unit.Contents, absent) {
			t.Fatalf("the take-over unit carries %s, so something other than the box could start it again:\n%s", absent, unit.Contents)
		}
	}
	if !unit.StartsOnlyWhenAsked {
		t.Fatal("the take-over unit is not marked as one the package leaves alone")
	}
}

func TestTheBoxDaemonOnAMacRunsTheSameCommandAndComesBackAfterACrash(t *testing.T) {
	plist := boxDaemonNamed(t, BoxServiceName).Contents
	for _, fragment := range []string{
		"<key>Label</key>\n\t<string>kim.intern." + BoxServiceName + "</string>",
		"<array>\n\t\t<string>/opt/homebrew/opt/internkim/libexec/internkim</string>\n\t\t<string>box</string>\n\t</array>",
		"<key>RunAtLoad</key>\n\t<true/>",
		"<key>SuccessfulExit</key>\n\t\t<false/>",
		"<key>ThrottleInterval</key>\n\t<integer>30</integer>",
		"<key>StandardErrorPath</key>\n\t<string>" + CompanyHostLogPath + "/" + BoxServiceName + ".log</string>",
	} {
		if !strings.Contains(plist, fragment) {
			t.Fatalf("the box plist lacks %q:\n%s", fragment, plist)
		}
	}
	if strings.Contains(plist, "<key>UserName</key>") {
		t.Fatalf("the box plist runs as an account, and the box installs a company as root:\n%s", plist)
	}
}

func TestTheTakeOverDaemonOnAMacIsLoadedWithoutRunningAndNeverKeptAlive(t *testing.T) {
	plist := boxDaemonNamed(t, CompanyHostTakeOverServiceName).Contents
	if !strings.Contains(plist, "<key>RunAtLoad</key>\n\t<false/>") {
		t.Fatalf("the take-over plist runs when launchd loads it:\n%s", plist)
	}
	for _, absent := range []string{"KeepAlive", "LaunchOnlyOnce"} {
		if strings.Contains(plist, absent) {
			t.Fatalf("the take-over plist carries %s, so launchd would start it again by itself:\n%s", absent, plist)
		}
	}
	if !strings.Contains(plist, "<string>restore</string>\n\t\t<string>--taking-over</string>") {
		t.Fatalf("the take-over plist does not run the take-over:\n%s", plist)
	}
}

func TestEveryBoxServiceIsReachedByName(t *testing.T) {
	layout := LinuxCompanyHostLayout()
	for _, service := range append(CompanyHostServices(layout), CompanyHostBoxServices(layout)...) {
		if found, isDeclared := CompanyHostDeclaredServiceNamed(layout, service.Name); !isDeclared || found.Name != service.Name {
			t.Fatalf("%s is declared and not found by its name", service.Name)
		}
	}
	if _, isDeclared := CompanyHostDeclaredServiceNamed(layout, "nothing-declares-this"); isDeclared {
		t.Fatal("a name nothing declares was found")
	}
}
