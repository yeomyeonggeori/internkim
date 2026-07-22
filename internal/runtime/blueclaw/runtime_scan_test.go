package blueclaw

import (
	"encoding/json"
	"testing"
)

func TestGeneratedRuntimeSchemasRequiredSubsetOfProperties(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocument("test-model")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var parsed any
	if errorValue := json.Unmarshal([]byte(document), &parsed); errorValue != nil {
		t.Fatal(errorValue)
	}
	var walk func(node any, trail string)
	walk = func(node any, trail string) {
		if documentNode, isDocument := node.(map[string]any); isDocument {
			required, _ := documentNode["required"].([]any)
			properties, hasProperties := documentNode["properties"].(map[string]any)
			if len(required) > 0 {
				for _, requiredValue := range required {
					name, _ := requiredValue.(string)
					if !hasProperties {
						t.Errorf("%s: required %q with no properties", trail, name)
						continue
					}
					if _, isDefined := properties[name]; !isDefined {
						t.Errorf("%s: required %q missing from properties", trail, name)
					}
				}
			}
			for key, value := range documentNode {
				walk(value, trail+"/"+key)
			}
			return
		}
		if values, isValues := node.([]any); isValues {
			for _, value := range values {
				name := ""
				if item, isItem := value.(map[string]any); isItem {
					name, _ = item["name"].(string)
				}
				walk(value, trail+"["+name+"]")
			}
		}
	}
	walk(parsed, "")
}
