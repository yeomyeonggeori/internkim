package capabilityprotocol

import (
	"encoding/json"

	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol/jsonschema"
)

func ToolInvokeOutputSchema() json.RawMessage {
	resourceEffectSchema := jsonschema.Object(
		jsonschema.Required("objectType", jsonschema.String()),
		jsonschema.Required("effect", jsonschema.String()),
		jsonschema.Field("id", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("url", jsonschema.String()),
		jsonschema.Field("visibility", jsonschema.String()),
		jsonschema.Field("durability", jsonschema.String()),
		jsonschema.Field("filename", jsonschema.String()),
		jsonschema.Field("contentType", jsonschema.String()),
		jsonschema.Field("summary", jsonschema.String()),
	)
	return jsonschema.Object(
		jsonschema.Required("provider", jsonschema.String()),
		jsonschema.Required("selectedBackend", jsonschema.String()),
		jsonschema.Required("toolName", jsonschema.String()),
		jsonschema.Field("outcome", jsonschema.StringEnum(string(ToolOutcomeSucceeded), string(ToolOutcomeFailed), string(ToolOutcomeDenied))),
		jsonschema.Field("effects", jsonschema.Array(resourceEffectSchema)),
		jsonschema.Field("status", jsonschema.String()),
		jsonschema.Field("content", jsonschema.String()),
		jsonschema.Field("isError", jsonschema.Boolean()),
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("errorCode", jsonschema.String()),
		jsonschema.Field("failureStage", jsonschema.String()),
		jsonschema.Field("retryable", jsonschema.Boolean()),
		jsonschema.Field("safeRetry", jsonschema.Boolean()),
		jsonschema.Required("result", jsonschema.Raw(json.RawMessage(`{}`))),
	).WithDialect(jsonschema.Draft202012).RawMessage()
}
