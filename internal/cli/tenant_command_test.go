package cli

import "testing"

func TestParseTenantProvisionOptionsAcceptsProvisionFlagsAndMembers(t *testing.T) {
	options, errorValue := parseTenantProvisionOptions([]string{
		"--tenant", "pilot-01",
		"--display-name", "Pilot",
		"--base", "/tmp/tenants",
		"--runtime", "host",
		"--assigned-host", "host-1",
		"--public-url", "https://pilot-01.example.com",
		"--mirror-host", "mirror-1",
		"--profile", "cloud-shared",
		"--template-rootfs", "/tmp/rootfs",
		"--nspawn-dir", "/tmp/nspawn",
		"--systemd-dir", "/tmp/systemd",
		"--bin-dir", "/tmp/bin",
		"--gateway-url", "https://gateway.example.com/v1/chat/completions",
		"--device-token", "device-token",
		"--gateway-shared-secret", "gateway-secret",
		"--release-download-token", "release-token",
		"--admin-password", "admin-password",
		"--admin-email", "admin@example.com",
		"--model", "model-name",
		"--base-url-template", "http://127.0.0.1:{port}",
		"--blueclaw-url-template", "http://127.0.0.1:{blueclawPort}",
		"--public-url-template", "https://{tenant}.example.com",
		"--port-start", "18065",
		"--language", "ko",
		"--token-output-root", "/tmp/output",
		"--account-id", "account-id",
		"--tunnel-id", "tunnel-id",
		"--api-token-path", "/tmp/cloudflare-token",
		"--api-base-url", "https://api.example.com",
		"--hostname-template", "{tenant}.example.com",
		"--rootfs-image", "/tmp/rootfs.ext4",
		"--workspace-image", "/tmp/workspace.ext4",
		"--port-base", "22000",
		"--member", "owner@example.com:Owner:owner-password",
		"--member", "generated@example.com:Generated",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if options.TenantID != "pilot-01" || options.DisplayName != "Pilot" || options.BasePath != "/tmp/tenants" {
		t.Fatalf("unexpected tenant provision basics: %+v", options)
	}
	if options.Runtime != "host" || options.SystemdDirectoryPath != "/tmp/systemd" || options.HostPortBase != 22000 {
		t.Fatalf("unexpected tenant provision runtime options: %+v", options)
	}
	if options.HostRootFilesystemImagePath != "/tmp/rootfs.ext4" || options.HostWorkspaceImagePath != "/tmp/workspace.ext4" {
		t.Fatalf("unexpected tenant provision host images: %+v", options)
	}
	if options.MattermostPublicURLTemplate != "https://{tenant}.example.com" || options.CloudflarePublicHostnameTemplate != "{tenant}.example.com" {
		t.Fatalf("unexpected tenant provision URL templates: %+v", options)
	}
	if len(options.Members) != 2 {
		t.Fatalf("expected two members, got %+v", options.Members)
	}
	if options.Members[0].Email != "owner@example.com" || options.Members[0].Password != "owner-password" || options.Members[0].IsPasswordGenerated {
		t.Fatalf("unexpected explicit member parse: %+v", options.Members[0])
	}
	if options.Members[1].Email != "generated@example.com" || options.Members[1].Password == "" || !options.Members[1].IsPasswordGenerated {
		t.Fatalf("unexpected generated member parse: %+v", options.Members[1])
	}

	manifest, errorValue := tenantProvisionManifest(options)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.MattermostInstance.PublicURL != "https://pilot-01.example.com" || manifest.MattermostInstance.InternalURL != "http://127.0.0.1:18065" {
		t.Fatalf("unexpected provision Mattermost instance: %+v", manifest.MattermostInstance)
	}
}

func TestTenantProvisionRuntimeStepSelection(t *testing.T) {
	hostSteps := tenantProvisionStepNames("host")
	expectedHostSteps := []string{"create", "install-host-runtime", "bootstrap-mattermost", "sync-cloudflare-tunnel"}
	if !equalStrings(hostSteps, expectedHostSteps) {
		t.Fatalf("unexpected host steps: %+v", hostSteps)
	}

	nspawnSteps := tenantProvisionStepNames("nspawn")
	expectedNspawnSteps := []string{"create", "install-container", "bootstrap", "start", "bootstrap-mattermost", "sync-cloudflare-tunnel"}
	if !equalStrings(nspawnSteps, expectedNspawnSteps) {
		t.Fatalf("unexpected nspawn steps: %+v", nspawnSteps)
	}
}

func TestParseTenantProvisionMemberRejectsInvalidFormat(t *testing.T) {
	for _, value := range []string{
		"missing-name@example.com",
		":Name:password",
		"invalid-email:Name:password",
		"person@example.com::password",
		"person@example.com:Name:password:extra",
	} {
		if _, errorValue := parseTenantProvisionMember(value); errorValue == nil {
			t.Fatalf("expected invalid member value to fail: %q", value)
		}
	}
}
