package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The relay asks for /runs/api. A mux that knows only the subtree answers that
// with a redirect to /runs/api/, and the switch inside had no case for the
// spelling the redirect arrives in, so the run list 404ed for everyone reading
// it through the app. The list answers where it is asked, at either spelling.
func TestTaskRunListIsServedAtBothSpellingsWithoutARedirect(t *testing.T) {
	service := &Service{}
	router := service.router()

	for _, path := range []string{"/runs/api", "/runs/api/"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code == http.StatusMovedPermanently || recorder.Code == http.StatusTemporaryRedirect {
			t.Fatalf("%s was redirected to %q instead of being served", path, recorder.Header().Get("Location"))
		}
		if recorder.Code == http.StatusNotFound {
			t.Fatalf("%s answered 404; the run list has no home there", path)
		}
	}
}

// Every path the switch sends to the list has to be one the mux serves, or the
// case is dead and the request 404s on the way in.
func TestTaskRunListPathsAreRoutedToTheTaskRunHandler(t *testing.T) {
	service := &Service{}
	router := service.router()

	for _, path := range []string{"/runs/api", "/runs/api/"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s answered %d; an unauthenticated read of the run list is 401", path, recorder.Code)
		}
	}
}
