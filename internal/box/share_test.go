package box

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAServiceBoxHoldsOnToItsShareUntilItServesAndThenHandsItOn(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	kept := []ShareKeeping{}
	daemon.KeepTheShare = func(_ context.Context, keeping ShareKeeping) {
		kept = append(kept, keeping)
	}

	runSteps(t, daemon, 1)
	if errorValue := MarkInstalledCompany(places.StateDirectoryPath, sampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	runSteps(t, daemon, 1)

	if len(kept) != 2 || kept[0].Session.Role != RoleService || !kept[0].IsStartingTheCompany || kept[1].IsStartingTheCompany {
		t.Fatalf("a service box kept its share as %+v", kept)
	}
}

func TestABoxReportsItsShareSignedByItsKeyAndReadsWhereThePiecesGo(t *testing.T) {
	identity := freshTestIdentity(t)
	keptThrough := time.Date(2026, 11, 22, 13, 0, 0, 0, time.UTC)
	var reported struct{ Report *ShareReport }
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/box/share" || !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			http.Error(writer, "not a signed share report", http.StatusUnauthorized)
			return
		}
		json.NewDecoder(request.Body).Decode(&reported)
		writer.Write([]byte(`{"standbysDigest":"d1","standbys":[{"publicKey":"b","keptThrough":"2026-11-22T13:00:00Z"},{"publicKey":"c","keptThrough":null}]}`))
	}))
	defer server.Close()

	placement, errorValue := Client{AppURL: server.URL}.ReportShare(context.Background(), identity, &ShareReport{RankedAgainst: "d0", KeptThrough: &keptThrough, SharePieces: 2, HeldPieces: 2, LogPieces: 4})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reported.Report == nil || reported.Report.RankedAgainst != "d0" || !reported.Report.KeptThrough.Equal(keptThrough) || reported.Report.LogPieces != 4 {
		t.Fatalf("the plane was told %+v", reported.Report)
	}
	if placement.StandbysDigest != "d1" || len(placement.Standbys) != 2 || placement.Standbys[1].KeptThrough != nil || !placement.Standbys[0].KeptThrough.Equal(keptThrough) {
		t.Fatalf("the placement read %+v", placement)
	}
}
