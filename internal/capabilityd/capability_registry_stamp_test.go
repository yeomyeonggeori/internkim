package capabilityd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestTheRegistryStampsTheToolsThePrintedContractNames(t *testing.T) {
	printedDescriptors := roundTrippedDescriptors(t, blueclawruntime.CurrentCapabilityContract().ToolDescriptors)
	stampedDescriptors := capabilities.RegistryDescriptors(servedCapabilityRegistry(t))

	printedNames, stampedNames := sortedToolNames(printedDescriptors), sortedToolNames(stampedDescriptors)
	if !reflect.DeepEqual(printedNames, stampedNames) {
		t.Fatalf("--print-capabilities names %v and the registry stamps %v", printedNames, stampedNames)
	}
	for index, printed := range printedDescriptors {
		if !reflect.DeepEqual(printed, stampedDescriptors[index]) {
			t.Fatalf("%s is printed as %+v and stamped as %+v", printed.Name, printed, stampedDescriptors[index])
		}
	}
}

func servedCapabilityRegistry(t *testing.T) capabilities.RegistryResponse {
	t.Helper()
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	Service{}.router().ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("the capability registry answered %d", responseRecorder.Code)
	}
	var registry capabilities.RegistryResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &registry); errorValue != nil {
		t.Fatal(errorValue)
	}
	return registry
}

func roundTrippedDescriptors(t *testing.T, descriptors []capabilities.Descriptor) []capabilities.Descriptor {
	t.Helper()
	document, errorValue := json.Marshal(descriptors)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var decoded []capabilities.Descriptor
	if errorValue := json.Unmarshal(document, &decoded); errorValue != nil {
		t.Fatal(errorValue)
	}
	return decoded
}

func sortedToolNames(descriptors []capabilities.Descriptor) []string {
	names := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		names = append(names, descriptor.Name)
	}
	sort.Strings(names)
	return names
}
