package capabilityprotocol

import (
	"encoding/json"
	"errors"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
	"reflect"
	"strings"
)

func ProjectResourceEffects(contract *ToolResultContract, result json.RawMessage) ([]ResourceEffect, error) {
	if contract == nil {
		return nil, errors.New("tool result contract is required")
	}
	var decoded any
	if errorValue := json.Unmarshal(result, &decoded); errorValue != nil {
		return nil, errors.New("tool result must be an object")
	}
	document, isObject := jsonschema.DocumentWithNullAsAbsent(decoded).(map[string]any)
	if !isObject {
		return nil, errors.New("tool result must be an object")
	}
	effects := []ResourceEffect{}
	for _, effectContract := range contract.Effects {
		if !effectConditionMatches(effectContract.When, document) {
			continue
		}
		if effectContract.EffectIdentity == ResourceEffectIdentitySingleton {
			effects = append(effects, ResourceEffect{ObjectType: strings.TrimSpace(effectContract.ObjectType), Effect: strings.TrimSpace(effectContract.Effect)})
			continue
		}
		identities, errorValue := resultEffectIdentities(document[effectContract.ResultField])
		if errorValue != nil {
			return nil, errorValue
		}
		for _, identity := range identities {
			effect, errorValue := projectResourceEffect(effectContract, identity)
			if errorValue != nil {
				return nil, errorValue
			}
			effects = append(effects, effect)
		}
	}
	return effects, nil
}

func effectConditionMatches(condition *EvidenceCondition, document map[string]any) bool {
	if condition == nil {
		return true
	}
	value, isPresent := document[condition.ResultField]
	if !isPresent {
		return false
	}
	var expectedValue any
	if json.Unmarshal(condition.Equals, &expectedValue) != nil {
		return false
	}
	return reflect.DeepEqual(value, expectedValue)
}

func resultEffectIdentities(value any) ([]string, error) {
	switch identity := value.(type) {
	case string:
		if normalizedIdentity := strings.TrimSpace(identity); normalizedIdentity != "" {
			return []string{normalizedIdentity}, nil
		}
	case []any:
		return resultEffectIdentityArray(identity)
	}
	return nil, errors.New("tool result effect identity is missing")
}

func resultEffectIdentityArray(values []any) ([]string, error) {
	identities := make([]string, 0, len(values))
	seenIdentities := map[string]bool{}
	for _, value := range values {
		identity, isString := value.(string)
		identity = strings.TrimSpace(identity)
		if !isString || identity == "" || seenIdentities[identity] {
			return nil, errors.New("tool result effect identity array is invalid")
		}
		seenIdentities[identity] = true
		identities = append(identities, identity)
	}
	if len(identities) == 0 {
		return nil, errors.New("tool result effect identity array is empty")
	}
	return identities, nil
}

func projectResourceEffect(contract ResourceEffectContract, identity string) (ResourceEffect, error) {
	effect := ResourceEffect{ObjectType: strings.TrimSpace(contract.ObjectType), Effect: strings.TrimSpace(contract.Effect)}
	switch contract.EffectIdentity {
	case ResourceEffectIdentityID:
		effect.ID = identity
	case ResourceEffectIdentityPath:
		effect.Path = identity
	case ResourceEffectIdentityURL:
		effect.URL = identity
	default:
		return ResourceEffect{}, errors.New("tool result effect identity is invalid")
	}
	return effect, nil
}
