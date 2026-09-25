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

func TestProjectResourceEffectsHonorsWhenConditions(t *testing.T) {
	contract := &ToolResultContract{
		Effects: []ResourceEffectContract{
			{ObjectType: "website", Effect: "previewed", ResultField: "previewURL", EffectIdentity: ResourceEffectIdentityURL, When: &EvidenceCondition{ResultField: "mode", Equals: json.RawMessage(`"preview"`)}},
			{ObjectType: "website", Effect: "published", ResultField: "publishedURL", EffectIdentity: ResourceEffectIdentityURL, When: &EvidenceCondition{ResultField: "mode", Equals: json.RawMessage(`"publish"`)}},
		},
	}

	previewEffects, errorValue := ProjectResourceEffects(contract, json.RawMessage(`{"mode":"preview","previewURL":"https://example.com/__preview/p-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(previewEffects) != 1 || previewEffects[0].Effect != "previewed" || previewEffects[0].URL != "https://example.com/__preview/p-1" {
		t.Fatalf("unexpected preview effects: %+v", previewEffects)
	}

	publishEffects, errorValue := ProjectResourceEffects(contract, json.RawMessage(`{"mode":"publish","publishedURL":"https://example.com"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(publishEffects) != 1 || publishEffects[0].Effect != "published" || publishEffects[0].URL != "https://example.com" {
		t.Fatalf("unexpected publish effects: %+v", publishEffects)
	}

	if _, errorValue := ProjectResourceEffects(contract, json.RawMessage(`{"mode":"publish"}`)); errorValue == nil {
		t.Fatal("expected a matched conditional effect with a missing identity to fail closed")
	}
}

func TestSingletonEffectNamesTheResourceWithoutAnIdentityField(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"changed":{"type":"boolean"}},"required":["changed"],"additionalProperties":false}`)
	singleton := ResourceEffectContract{ObjectType: "company settings", Effect: "updated", EffectIdentity: ResourceEffectIdentitySingleton, When: &EvidenceCondition{ResultField: "changed", Equals: json.RawMessage(`true`)}}
	misdeclared := singleton
	misdeclared.ResultField = "changed"
	if errorValue := validateResultContract(&ToolResultContract{Schema: schema, Effects: []ResourceEffectContract{singleton}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if validateResultContract(&ToolResultContract{Schema: schema, Effects: []ResourceEffectContract{misdeclared}}) == nil {
		t.Fatal("expected a singleton effect naming a resultField to be refused")
	}
	contract := &ToolResultContract{Schema: schema, Effects: []ResourceEffectContract{singleton}}
	changed, errorValue := ProjectResourceEffects(contract, json.RawMessage(`{"changed":true}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(changed) != 1 || changed[0] != (ResourceEffect{ObjectType: "company settings", Effect: "updated"}) {
		t.Fatalf("unexpected singleton effects: %+v", changed)
	}
	unchanged, errorValue := ProjectResourceEffects(contract, json.RawMessage(`{"changed":false}`))
	if errorValue != nil || len(unchanged) != 0 {
		t.Fatalf("expected no effect when nothing changed, got %+v %v", unchanged, errorValue)
	}
}
