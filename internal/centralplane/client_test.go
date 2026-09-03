package centralplane

import "testing"

func TestSettingsNeedEveryPart(t *testing.T) {
	complete := Settings{AppURL: "https://app", AgentAPIKey: "key", ProjectURL: "https://plane", PublishableKey: "publishable"}
	if !complete.Configured() {
		t.Fatal("a complete setting should be usable")
	}
	for _, missing := range []Settings{
		{AgentAPIKey: "key", ProjectURL: "https://plane", PublishableKey: "publishable"},
		{AppURL: "https://app", ProjectURL: "https://plane", PublishableKey: "publishable"},
		{AppURL: "https://app", AgentAPIKey: "key", PublishableKey: "publishable"},
		{AppURL: "https://app", AgentAPIKey: "key", ProjectURL: "https://plane"},
	} {
		if missing.Configured() {
			t.Fatalf("a setting missing a part must not be usable: %+v", missing)
		}
	}
}
