package blueclaw

import (
	"slices"
	"strings"
	"testing"
)

func TestTheEmbeddingServerRunsTheLibraryModelOnLoopbackAsItsOwnAccount(t *testing.T) {
	for _, layout := range []CompanyHostLayout{LinuxCompanyHostLayout(), MacCompanyHostLayout(testHomebrewPrefix)} {
		service, isDeclared := CompanyHostServiceNamed(layout, EmbeddingServiceName)
		if !isDeclared {
			t.Fatalf("the bundle declares no %s", EmbeddingServiceName)
		}
		command := strings.Join(service.Command, " ")
		for _, wanted := range []string{
			layout.EmbeddingServerPath() + " -m " + layout.EmbeddingModelPath(),
			"--embeddings", "--pooling mean",
			"--host 127.0.0.1 --port " + EmbeddingListenPort,
			"-c 2048 -b 2048 -ub 2048",
		} {
			if !strings.Contains(command, wanted) {
				t.Errorf("the embedding server starts as %q and lacks %q", command, wanted)
			}
		}
		if service.Account != EmbeddingUserName {
			t.Errorf("the embedding server runs as %q", service.Account)
		}
	}
}

func TestTheEmbeddingServerAndCapabilitydAgreeOnTheAddress(t *testing.T) {
	capability, _ := CompanyHostServiceNamed(LinuxCompanyHostLayout(), CapabilitydServiceName)
	index := slices.Index(capability.Command, "--embedding-url")
	if index < 0 || capability.Command[index+1] != "http://"+EmbeddingListenAddress {
		t.Fatalf("capabilityd is started as %v and does not reach the embedding server at %s", capability.Command, EmbeddingListenAddress)
	}
}

func TestTheEmbeddingAccountIsDeclaredAndTheServerStartsBeforeCapabilityd(t *testing.T) {
	if !strings.Contains(CompanyHostSysusersFile(), "u "+EmbeddingUserName+" - ") {
		t.Fatalf("sysusers declares no %s:\n%s", EmbeddingUserName, CompanyHostSysusersFile())
	}
	capabilityd := companyHostOrderingFor(CapabilitydServiceName)
	if !slices.Contains(capabilityd.After, EmbeddingServiceName+".service") || !slices.Contains(capabilityd.Wants, EmbeddingServiceName+".service") {
		t.Fatalf("capabilityd is ordered %+v and does not wait for the embedding server", capabilityd)
	}
	if slices.Contains(capabilityd.Requires, EmbeddingServiceName+".service") {
		t.Fatal("capabilityd serves tools that never embed, so a failed embedding server must not take it down")
	}
}

func TestTheEmbeddingServerIsNotGivenTheCompanysSecrets(t *testing.T) {
	service, _ := CompanyHostServiceNamed(LinuxCompanyHostLayout(), EmbeddingServiceName)
	for _, source := range service.Environment {
		if source.FilePath != "" {
			t.Fatalf("the embedding server reads %s, and an unprivileged account that reads files is one that can leak them", source.FilePath)
		}
	}
}
