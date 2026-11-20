package box

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

const connectionFileKey = "header.claims.signature"

func computerServingOnAConnectionFile(t *testing.T, plane *fakePlane) (Daemon, Places, *installs) {
	t.Helper()
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	writeConnectionFile(t, places.ConnectionFilePath, sampleConnectionDocument(t))
	for _, path := range places.CredentialPaths {
		if errorValue := os.WriteFile(path, []byte(connectionFileKey+"\n"), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return daemon, places, recorded
}

func requireCredentials(t *testing.T, places Places, accessToken string) {
	t.Helper()
	for _, path := range places.CredentialPaths {
		requireFile(t, path, accessToken+"\n", 0o640)
	}
	connection, errorValue := companyhost.ReadConnection(places.ConnectionFilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if connection.AgentKey != accessToken {
		t.Fatalf("the connection file carries %q, want %q", connection.AgentKey, accessToken)
	}
}

func TestAComputerOnAConnectionFileClaimsItselfWithTheFileKeyAndServesOnItsSessions(t *testing.T) {
	plane := &fakePlane{accessToken: "first.session.token"}
	daemon, places, recorded := computerServingOnAConnectionFile(t, plane)

	runSteps(t, daemon, 1)

	if !slices.Equal(plane.claimedWith, []string{connectionFileKey}) || !slices.Equal(plane.claimsReplacing, []bool{false}) {
		t.Fatalf("claimed with %v replacing %v, want once with the key the connection file holds and replacing nothing", plane.claimedWith, plane.claimsReplacing)
	}
	if len(recorded.requests) != 0 || plane.announcements != 0 {
		t.Fatalf("the computer reinstalled its company %d times and announced itself %d times, want neither", len(recorded.requests), plane.announcements)
	}
	if daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("the box holds %q as its installed company", daemon.installedCompany())
	}
	requireCredentials(t, places, "first.session.token")

	plane.accessToken = "second.session.token"
	runSteps(t, daemon, 1)

	if len(plane.claimedWith) != 1 {
		t.Fatalf("a box already on its own key claimed again: %v", plane.claimedWith)
	}
	requireCredentials(t, places, "second.session.token")
}

func TestAComputerWhoseClaimIsRefusedKeepsServingOnItsConnectionFileKey(t *testing.T) {
	plane := &fakePlane{refusesClaims: true}
	daemon, places, _ := computerServingOnAConnectionFile(t, plane)

	runSteps(t, daemon, 3)

	if len(plane.claimedWith) != 3 || plane.announcements != 0 {
		t.Fatalf("claims = %d, announcements = %d, want a claim every try and no announcement", len(plane.claimedWith), plane.announcements)
	}
	if daemon.installedCompany() != "" {
		t.Fatalf("a refused computer took %q as a box", daemon.installedCompany())
	}
	requireCredentials(t, places, connectionFileKey)
}

func TestAComputerWhoseClaimAnswerWasLostStillMovesOntoItsBoxKey(t *testing.T) {
	plane := &fakePlane{losesClaimAnswer: true, accessToken: "box.session.token"}
	daemon, places, _ := computerServingOnAConnectionFile(t, plane)

	runSteps(t, daemon, 1)

	if daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("the plane kept the claim and the computer holds %q", daemon.installedCompany())
	}
	requireCredentials(t, places, "box.session.token")
}

func TestAComputerWhoseOwnClaimSpentItsKeyFinishesTheMove(t *testing.T) {
	plane := &fakePlane{isClaimed: true, refusesClaims: true, accessToken: "box.session.token"}
	daemon, places, _ := computerServingOnAConnectionFile(t, plane)

	runSteps(t, daemon, 1)

	if daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("the plane answers this box as the service box and the computer holds %q", daemon.installedCompany())
	}
	requireCredentials(t, places, "box.session.token")
}

func TestAComputerThePlaneKeepsAsAStandbyStaysOnItsConnectionFile(t *testing.T) {
	plane := &fakePlane{role: RoleStandby, accessToken: "standby.session.token"}
	daemon, places, _ := computerServingOnAConnectionFile(t, plane)

	runSteps(t, daemon, 2)

	if daemon.installedCompany() != "" {
		t.Fatalf("a computer the plane keeps as a standby took %q as a box, which would stop the services it runs", daemon.installedCompany())
	}
	requireCredentials(t, places, connectionFileKey)
}

func TestAComputerThatCannotTellWhoseDataItHoldsClaimsNothing(t *testing.T) {
	plane := &fakePlane{accessToken: "box.session.token"}
	daemon, places, _ := computerServingOnAConnectionFile(t, plane)
	daemon.CompaniesHeld = func() (companyhost.CompaniesHeld, error) {
		return companyhost.CompaniesHeld{}, errors.New("the company directory is outside the companies root")
	}

	runSteps(t, daemon, 2)

	if len(plane.claimedWith) != 0 {
		t.Fatalf("claimed with %v before knowing it could serve as a box", plane.claimedWith)
	}
	requireCredentials(t, places, connectionFileKey)
}

func TestAComputerBesideOtherBoxesStaysOnItsConnectionFileAndAsksOnce(t *testing.T) {
	plane := &fakePlane{otherComputers: true}
	daemon, places, _ := computerServingOnAConnectionFile(t, plane)
	daemon.IsOnANetwork = func(context.Context) bool { return false }
	attempts := []bool{}
	daemon.GetOnline = func(ctx context.Context, hasCompany bool) error {
		attempts = append(attempts, hasCompany)
		return nil
	}

	runSteps(t, daemon, 3)

	if len(plane.claimedWith) != 1 || plane.announcements != 0 {
		t.Fatalf("claims = %d, announcements = %d, want one claim the plane refuses and no announcement", len(plane.claimedWith), plane.announcements)
	}
	if daemon.installedCompany() != "" {
		t.Fatalf("a computer beside other boxes took %q as a box", daemon.installedCompany())
	}
	requireCredentials(t, places, connectionFileKey)
	if len(attempts) < 3 || slices.Contains(attempts, false) {
		t.Fatalf("a computer staying on its connection file got online as %v, want each time as one holding a company", attempts)
	}
}

func TestAResetOfAComputerOnItsConnectionFileGivesBackTheFileKeyAndLeavesAnEmptyBox(t *testing.T) {
	plane := &fakePlane{otherComputers: true}
	daemon, _, _ := computerServingOnAConnectionFile(t, plane)
	askThroughTheCommand(t, daemon)

	runSteps(t, daemon, 2)

	if len(plane.claimedWith) != 0 {
		t.Fatalf("a computer asked to reset claimed itself with %v", plane.claimedWith)
	}
	if !slices.Equal(plane.disconnectedWith, []string{connectionFileKey}) {
		t.Fatalf("disconnected with %q, want once carrying the key the connection file holds", plane.disconnectedWith)
	}
	if daemon.connectedByFile() || plane.announcements != 2 {
		t.Fatalf("connected by file = %v, announcements = %d, want an empty box announcing itself", daemon.connectedByFile(), plane.announcements)
	}
	requireNoResetRequest(t, daemon)
}

func TestAnInstallFromAConnectionFileReplacesTheOtherComputers(t *testing.T) {
	plane := &fakePlane{otherComputers: true, accessToken: "file.session.token"}
	daemon, _ := daemonFor(t, plane, &installs{})

	if errorValue := daemon.InstallWithConnectionFile(context.Background(), "connection-file-key", "sk-or-v1-typed"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if !slices.Equal(plane.claimsReplacing, []bool{true}) {
		t.Fatalf("an install claimed replacing %v, want once replacing the company's other computers", plane.claimsReplacing)
	}
}
