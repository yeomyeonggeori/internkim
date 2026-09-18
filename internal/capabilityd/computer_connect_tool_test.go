package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestComputerConnectAsksTheDeviceForACodeAddressedToTheRequester(t *testing.T) {
	admind := answering(`{"code":"ABCD-1234","expiresAt":"2026-09-18T09:10:00Z","deepLink":"internkim://pair?device_url=https%3A%2F%2Fdevice.example.test&code=ABCD-1234","pairCommand":"internkim-companion pair --device-url https://device.example.test --code ABCD-1234"}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverHTTP: admind})

	answered, errorValue := service.invokeComputerConnectTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: computerConnectToolName,
		Input:    json.RawMessage(`{}`),
		Context: capabilityprotocol.ToolInvokeContext{
			Platform:                "buzz",
			RequesterPlatformUserID: "buzz-user-1",
			RequesterPersonID:       "person-1",
			RequesterEmail:          "Member@Example.com",
			RequesterName:           "이샘플",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectSucceeded(t, answered)

	asked := admind.Asked()
	if len(asked) != 1 || asked[0].Method != "POST" || asked[0].Path != "/_internkim/companion/pairing-codes" {
		t.Fatalf("asked = %+v", asked)
	}
	var owner companionPairingCodeOwner
	if errorValue := json.Unmarshal([]byte(asked[0].Body), &owner); errorValue != nil {
		t.Fatal(errorValue)
	}
	expected := companionPairingCodeOwner{OwnerPlatform: "buzz", OwnerPlatformUserID: "buzz-user-1", OwnerPersonID: "person-1", OwnerEmail: "member@example.com", OwnerName: "이샘플"}
	if owner != expected {
		t.Fatalf("owner = %+v, expected %+v", owner, expected)
	}

	var result computerConnectResult
	if errorValue := json.Unmarshal(answered.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Code != "ABCD-1234" || result.ExpiresAt != "2026-09-18T09:10:00Z" {
		t.Fatalf("result = %+v", result)
	}
	if result.ServiceCommand != "internkim-companion service install" || result.DocumentationURL != "https://docs.intern.kim/docs/companion" {
		t.Fatalf("result = %+v", result)
	}
}

func TestComputerConnectRefusesACallThatNamesNobody(t *testing.T) {
	admind := answering(`{}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverHTTP: admind})

	_, errorValue := service.invokeComputerConnectTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: computerConnectToolName,
		Input:    json.RawMessage(`{}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "named nobody") {
		t.Fatalf("error = %v", errorValue)
	}
	if len(admind.Asked()) != 0 {
		t.Fatal("the device was asked for a code nobody could use")
	}
}

func TestComputerConnectCarriesTheDeviceRefusal(t *testing.T) {
	admind := answeringPerCall(func(*http.Request) (int, string) { return http.StatusForbidden, "local access required" })
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverHTTP: admind})

	_, errorValue := service.invokeComputerConnectTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: computerConnectToolName,
		Input:    json.RawMessage(`{}`),
		Context:  capabilityprotocol.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "local access required") {
		t.Fatalf("error = %v", errorValue)
	}
}
