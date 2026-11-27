package walstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const keptCompanyID = "00000000-0000-4000-8000-0000000000c1"

type keptPieceNames struct {
	mutex        sync.Mutex
	names        []string
	listRequests int
}

func (kept *keptPieceNames) keepRun(timeline int32, start pglogrepl.LSN, count int) pglogrepl.LSN {
	for range count {
		name := PieceName{Timeline: timeline, Start: start, End: start + 0x80}
		kept.names = append(kept.names, name.String())
		start = name.End
	}
	return start
}

func (kept *keptPieceNames) serve(t *testing.T) offsite.Store {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.EscapedPath(), "/storage/v1")
		if request.Method != http.MethodPost || path != "/object/list/"+offsite.Bucket {
			t.Errorf("unexpected %s %s", request.Method, path)
			return
		}
		kept.list(writer, request)
	}))
	t.Cleanup(server.Close)
	return offsite.Store{ProjectURL: server.URL, PublishableKey: "publishable", AccessToken: func() (string, error) { return "host.session.token", nil }}
}

func (kept *keptPieceNames) list(writer http.ResponseWriter, request *http.Request) {
	var asked struct {
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		SortBy struct {
			Order string `json:"order"`
		} `json:"sortBy"`
	}
	json.NewDecoder(request.Body).Decode(&asked)
	kept.mutex.Lock()
	defer kept.mutex.Unlock()
	kept.listRequests++
	ordered := slices.Sorted(slices.Values(kept.names))
	if asked.SortBy.Order == "desc" {
		slices.Reverse(ordered)
	}
	page := []offsite.StoredObject{}
	for index := asked.Offset; index < len(ordered) && index < asked.Offset+asked.Limit; index++ {
		page = append(page, offsite.StoredObject{Name: ordered[index]})
	}
	json.NewEncoder(writer).Encode(page)
}

func (kept *keptPieceNames) pieces(t *testing.T) KeptPieces {
	return KeptPieces{Destination: offsite.Destination{CompanyID: keptCompanyID, Store: kept.serve(t)}, SystemID: "7400000000000000001"}
}

func TestARestartedFollowerFindsTheKeptEndInOnePageHoweverMuchIsKept(t *testing.T) {
	kept := &keptPieceNames{}
	end := kept.keepRun(1, 0x1000000, 2500)
	newest, isFound, errorValue := kept.pieces(t).NewestEnd(context.Background(), 1)
	if errorValue != nil || !isFound || newest != end {
		t.Fatalf("the kept WAL ends at %s, found %s (found=%v): %v", end, newest, isFound, errorValue)
	}
	if kept.listRequests != 1 {
		t.Errorf("finding where 2500 kept pieces end listed %d pages, and the company database holds every commit until it is done", kept.listRequests)
	}
}

func TestTheKeptEndIsTheNewestPieceOnTheAskedTimeline(t *testing.T) {
	kept := &keptPieceNames{}
	firstTimelineEnd := kept.keepRun(1, 0x1000000, 3)
	secondTimelineEnd := kept.keepRun(2, firstTimelineEnd-0x80, 3)
	pieces := kept.pieces(t)
	for _, expected := range []struct {
		timeline int32
		end      pglogrepl.LSN
		isFound  bool
	}{
		{timeline: 1, end: firstTimelineEnd, isFound: true},
		{timeline: 2, end: secondTimelineEnd, isFound: true},
		{timeline: 3},
	} {
		newest, isFound, errorValue := pieces.NewestEnd(context.Background(), expected.timeline)
		if errorValue != nil || isFound != expected.isFound || newest != expected.end {
			t.Errorf("timeline %d: found %s (found=%v), expected %s (found=%v): %v", expected.timeline, newest, isFound, expected.end, expected.isFound, errorValue)
		}
	}
}
