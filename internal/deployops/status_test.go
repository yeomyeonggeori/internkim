package deployops

import "testing"

func TestEnrichEndpointStatusReadsReleaseSetState(t *testing.T) {
	status := EndpointStatus{State: "ok"}
	body := []byte(`{"state":"update_available","updateAllowed":true,"current":{"releaseID":"release-current"},"latest":{"releaseID":"release-latest"}}`)

	enrichEndpointStatus(&status, body)

	if status.State != "update_available" {
		t.Fatalf("expected release state, got %q", status.State)
	}
	if status.CurrentRelease != "release-current" || status.LatestRelease != "release-latest" {
		t.Fatalf("unexpected release ids: %#v", status)
	}
	if !status.UpdateAllowed {
		t.Fatal("expected update to be allowed")
	}
}
