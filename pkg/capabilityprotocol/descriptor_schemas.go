package capabilityprotocol

import (
	"encoding/json"

	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol/jsonschema"
)

func TextLLMInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("model", jsonschema.String()),
		jsonschema.Field("provider", jsonschema.String()),
		jsonschema.Field("accelerator", jsonschema.String()),
		jsonschema.Field("executionMode", jsonschema.String()),
		jsonschema.Required("messages", jsonschema.Array(llmMessageSchema())),
		jsonschema.Field("requireParameters", jsonschema.Boolean()),
		jsonschema.Field("enableResponseHealing", jsonschema.Boolean()),
	).WithDialect(jsonschema.Draft202012).RawMessage()
}

func StructuredLLMInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("model", jsonschema.String()),
		jsonschema.Field("provider", jsonschema.String()),
		jsonschema.Field("accelerator", jsonschema.String()),
		jsonschema.Field("executionMode", jsonschema.String()),
		jsonschema.Required("messages", jsonschema.Array(llmMessageSchema())),
		jsonschema.Required("structuredOutputSchema", jsonschema.Object(
			jsonschema.Required("name", jsonschema.String()),
			jsonschema.Required("document", jsonschema.Raw(json.RawMessage(`{}`))),
			jsonschema.Field("isStrictlyEnforced", jsonschema.Boolean()),
		)),
		jsonschema.Field("requireParameters", jsonschema.Boolean()),
		jsonschema.Field("enableResponseHealing", jsonschema.Boolean()),
	).WithDialect(jsonschema.Draft202012).RawMessage()
}

func EmbeddingInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("input", jsonschema.Raw(json.RawMessage(`{}`))),
		jsonschema.Field("model", jsonschema.String()),
		jsonschema.Field("provider", jsonschema.String()),
		jsonschema.Field("executionMode", jsonschema.String()),
		jsonschema.Field("task", jsonschema.String()),
		jsonschema.Field("inputType", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
		jsonschema.Field("outputDimensions", jsonschema.SafeInteger()),
	).WithDialect(jsonschema.Draft202012).RawMessage()
}

func llmMessageSchema() jsonschema.Schema {
	return jsonschema.Object(
		jsonschema.Required("role", jsonschema.String()),
		jsonschema.Field("content", jsonschema.String()),
		jsonschema.Field("parts", jsonschema.Array(jsonschema.Object(
			jsonschema.Required("type", jsonschema.String()),
			jsonschema.Field("text", jsonschema.String()),
			jsonschema.Field("mimeType", jsonschema.String()),
			jsonschema.Field("dataBase64", jsonschema.String()),
		))),
	)
}

func attentionTriageInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("jobID", jsonschema.String()),
		jsonschema.Required("toolName", jsonschema.String()),
		jsonschema.Required("status", jsonschema.String()),
		jsonschema.Field("privacyClass", jsonschema.String()),
		jsonschema.Field("watchAttemptCount", jsonschema.SafeInteger()),
		jsonschema.Field("lastAttentionAt", jsonschema.String()),
		jsonschema.Field("createdAt", jsonschema.String()),
		jsonschema.Field("updatedAt", jsonschema.String()),
		jsonschema.Field("expiresAt", jsonschema.String()),
		jsonschema.Field("error", jsonschema.String()),
		jsonschema.Field("denialCode", jsonschema.String()),
	).WithDialect(jsonschema.Draft202012).RawMessage()
}

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
