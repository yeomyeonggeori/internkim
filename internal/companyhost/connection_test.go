package companyhost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func validConnectionDocument() map[string]any {
	return map[string]any{
		"schemaVersion": 1,
		"appURL":        "https://company.example.com",
		"company":       map[string]any{"id": "00000000-0000-4000-8000-000000000001", "name": "Example Co", "slug": "example"},
		"centralPlane":  map[string]any{"projectURL": "https://project.supabase.co", "publishableKey": "publishable"},
		"gatewayURL":    "wss://gateway.example.com",
		"agentKey":      strings.Repeat("a", 64),
	}
}

func documentWith(change func(document map[string]any)) []byte {
	document := validConnectionDocument()
	change(document)
	encoded, _ := json.Marshal(document)
	return encoded
}

func TestParseConnectionAcceptsTheFileCompanySetupIssues(t *testing.T) {
	connection, errorValue := ParseConnection(documentWith(func(map[string]any) {}))
	if errorValue != nil {
		t.Fatalf("parse the issued connection file: %v", errorValue)
	}
	if connection.Company.Slug != "example" || connection.AgentKey != strings.Repeat("a", 64) {
		t.Fatalf("connection read back wrong: %+v", connection)
	}
}

func TestTheGatewayIsDialledAsASocketWhateverSchemeCompanySetupIssued(t *testing.T) {
	cases := map[string]string{
		"https://gateway.example.com":  "wss://gateway.example.com",
		"https://gateway.example.com/": "wss://gateway.example.com",
		"http://127.0.0.1:8787":        "ws://127.0.0.1:8787",
		"wss://gateway.example.com":    "wss://gateway.example.com",
		"ws://127.0.0.1:8787":          "ws://127.0.0.1:8787",
	}
	for issued, dialled := range cases {
		t.Run(issued, func(t *testing.T) {
			connection, errorValue := ParseConnection(documentWith(func(document map[string]any) { document["gatewayURL"] = issued }))
			if errorValue != nil {
				t.Fatalf("refused %s: %v", issued, errorValue)
			}
			if connection.GatewayURL != dialled {
				t.Fatalf("%s became %s, want %s", issued, connection.GatewayURL, dialled)
			}
			if got := relayEnvironment(connection)[3]; got.Name != "GATEWAY_URL" || got.Value != dialled {
				t.Fatalf("the relay is handed %+v, want GATEWAY_URL=%s", got, dialled)
			}
		})
	}
}

func TestTheGatewayProductionIssuesIsOneTheHostAccepts(t *testing.T) {
	manifest, errorValue := os.ReadFile(filepath.Join("..", "..", ".monkeys"))
	if errorValue != nil {
		t.Fatalf("read the vault manifest: %v", errorValue)
	}
	declared := regexp.MustCompile(`(?m)^GATEWAY_URL=(\S+)$`).FindStringSubmatch(string(manifest))
	if declared == nil {
		t.Fatal("the vault manifest no longer declares GATEWAY_URL")
	}
	if _, errorValue := ParseConnection(documentWith(func(document map[string]any) { document["gatewayURL"] = declared[1] })); errorValue != nil {
		t.Fatalf("company setup issues %s and the host refuses it: %v", declared[1], errorValue)
	}
}

func TestParseConnectionRefusesEveryShapeCompanySetupNeverIssues(t *testing.T) {
	cases := map[string][]byte{
		"a later schema version": documentWith(func(document map[string]any) { document["schemaVersion"] = 2 }),
		"a version as text":      []byte(`{"schemaVersion":"1"}`),
		"a version as a decimal": []byte(`{"schemaVersion":1.0}`),
		"a version as a boolean": []byte(`{"schemaVersion":true}`),
		"an address with a fragment": documentWith(func(document map[string]any) {
			document["appURL"] = "https://company.example.com/#fragment"
		}),
		"an address carrying credentials": documentWith(func(document map[string]any) {
			document["appURL"] = "https://user:pass@company.example.com"
		}),
		"an address with a newline": documentWith(func(document map[string]any) {
			document["appURL"] = "https://company.example.com\nmalicious"
		}),
		"a gateway that is not a web address": documentWith(func(document map[string]any) {
			document["gatewayURL"] = "ftp://gateway.example.com"
		}),
		"an upper case company key": documentWith(func(document map[string]any) {
			document["agentKey"] = strings.Repeat("A", 64)
		}),
		"a short company key": documentWith(func(document map[string]any) {
			document["agentKey"] = strings.Repeat("a", 63)
		}),
		"a publishable key that is not text": documentWith(func(document map[string]any) {
			document["centralPlane"] = map[string]any{"projectURL": "https://project.supabase.co", "publishableKey": 7}
		}),
		"a company id that is not a uuid": documentWith(func(document map[string]any) {
			document["company"] = map[string]any{"id": "not-a-uuid", "name": "Example Co", "slug": "example"}
		}),
		"a company that is not an object":  documentWith(func(document map[string]any) { document["company"] = []any{} }),
		"a field the schema does not name": documentWith(func(document map[string]any) { document["extra"] = "value" }),
		"an empty document":                []byte(`{}`),
		"text that is not json":            []byte(`not json`),
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			if _, errorValue := ParseConnection(document); errorValue == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

func TestConnectionCarriesEveryFieldTheCentralPlaneSchemaNames(t *testing.T) {
	source, errorValue := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "company", "host-setup.ts"))
	if errorValue != nil {
		t.Fatalf("read the central plane schema: %v", errorValue)
	}
	published := topLevelSchemaFields(t, string(source))
	carried := jsonFieldNames(reflect.TypeOf(Connection{}))
	if !reflect.DeepEqual(published, carried) {
		t.Fatalf("the connection file schema drifted: the central plane issues %v, this binary reads %v", published, carried)
	}
}

func topLevelSchemaFields(t *testing.T, source string) []string {
	t.Helper()
	start := strings.Index(source, "export const hostConfigurationSchema = z.object({")
	if start < 0 {
		t.Fatal("hostConfigurationSchema is no longer declared in the central plane schema")
	}
	end := strings.Index(source[start:], "\n}).strict();")
	if end < 0 {
		t.Fatal("hostConfigurationSchema is no longer a strict object")
	}
	fields := regexp.MustCompile(`(?m)^\t(\w+):`).FindAllStringSubmatch(source[start:start+end], -1)
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field[1])
	}
	sort.Strings(names)
	return names
}

func jsonFieldNames(structure reflect.Type) []string {
	names := make([]string, 0, structure.NumField())
	for index := range structure.NumField() {
		names = append(names, structure.Field(index).Tag.Get("json"))
	}
	sort.Strings(names)
	return names
}
