package capabilityd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestTheLiveRegistryServesTheDefaultToolDescriptors(t *testing.T) {
	stampedContractDescriptors := roundTrippedDescriptors(t, capabilities.DefaultToolDescriptors())
	servedDescriptors := capabilities.RegistryDescriptors(servedCapabilityRegistry(t))

	stampedNames, servedNames := sortedToolNames(stampedContractDescriptors), sortedToolNames(servedDescriptors)
	if !reflect.DeepEqual(stampedNames, servedNames) {
		t.Fatalf("the default descriptors name %v and the live registry serves %v", stampedNames, servedNames)
	}
	for index, stamped := range stampedContractDescriptors {
		if !reflect.DeepEqual(stamped, servedDescriptors[index]) {
			t.Fatalf("%s is declared as %+v and served as %+v", stamped.Name, stamped, servedDescriptors[index])
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
