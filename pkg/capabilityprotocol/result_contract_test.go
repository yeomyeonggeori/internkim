package capabilityprotocol

import (
	"encoding/json"
	"testing"
)

func TestProjectResourceEffectsUsesEveryCanonicalIdentity(t *testing.T) {
	contract := &ToolResultContract{
		Effects: []ResourceEffectContract{
			{ObjectType: "website", Effect: "published", ResultField: "siteID", EffectIdentity: ResourceEffectIdentityID},
			{ObjectType: "website", Effect: "published", ResultField: "publishedURL", EffectIdentity: ResourceEffectIdentityURL},
		},
	}

	effects, errorValue := ProjectResourceEffects(contract, json.RawMessage(`{"siteID":"site-1","publishedURL":"https://example.com"}`))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(effects) != 2 || effects[0].ID != "site-1" || effects[1].URL != "https://example.com" {
		t.Fatalf("unexpected effects: %+v", effects)
	}
}

func TestProjectResourceEffectsFailsClosed(t *testing.T) {
	contract := &ToolResultContract{Effects: []ResourceEffectContract{{
		ObjectType:     "file",
		Effect:         "updated",
		ResultField:    "paths",
		EffectIdentity: ResourceEffectIdentityPath,
	}}}
	for _, result := range []json.RawMessage{
		json.RawMessage(`{"paths":[]}`),
		json.RawMessage(`{"paths":["/workspace/a","/workspace/a"]}`),
		json.RawMessage(`{"paths":[1]}`),
		json.RawMessage(`{}`),
	} {
		if _, errorValue := ProjectResourceEffects(contract, result); errorValue == nil {
			t.Fatalf("expected invalid result rejection: %s", result)
		}
	}
}
