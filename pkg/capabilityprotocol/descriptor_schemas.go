package capabilityprotocol

import (
	"encoding/json"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

func browserHandoffInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("url", jsonschema.String()),
		jsonschema.Field("message", jsonschema.String()),
	).RawMessage()
}

func browserFillInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("target", jsonschema.String()),
		jsonschema.Field("ref", jsonschema.String()),
		jsonschema.Field("selector", jsonschema.String()),
		jsonschema.Required("text", jsonschema.String()),
	).RawMessage()
}

func browserSelectInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("target", jsonschema.String()),
		jsonschema.Field("ref", jsonschema.String()),
		jsonschema.Field("selector", jsonschema.String()),
		jsonschema.Required("value", jsonschema.String()),
	).RawMessage()
}

func browserPressInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Required("key", jsonschema.String())).RawMessage()
}

func browserWaitInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("target", jsonschema.String()),
		jsonschema.Field("ref", jsonschema.String()),
		jsonschema.Field("selector", jsonschema.String()),
		jsonschema.Field("milliseconds", jsonschema.Integer()),
	).RawMessage()
}

func userConfirmInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("message", jsonschema.String()),
		jsonschema.Field("reason", jsonschema.String()),
	).RawMessage()
}

func userInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("message", jsonschema.String()),
		jsonschema.Field("placeholder", jsonschema.String()),
	).RawMessage()
}

func filePickInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("accept", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("multiple", jsonschema.Boolean()),
	).RawMessage()
}

func emptyToolInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func mountCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("displayName", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
	).RawMessage()
}

func mountReferenceInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Field("mountID", jsonschema.String())).RawMessage()
}

func mountPathInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
	).RawMessage()
}

func mountWriteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("content", jsonschema.String()),
		jsonschema.Field("contentBase64", jsonschema.String()),
	).RawMessage()
}

func mountRenameInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("toPath", jsonschema.String()),
	).RawMessage()
}

func mountDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("recursive", jsonschema.Boolean()),
	).RawMessage()
}

func mountTruncateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("sizeBytes", jsonschema.Integer()),
	).RawMessage()
}

func mountChangeModeInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("mode", jsonschema.Integer()),
	).RawMessage()
}

func mountWatchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mountID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("sinceUnixNano", jsonschema.Integer()),
	).RawMessage()
}

func TextLLMInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("model", jsonschema.String()),
		jsonschema.Field("provider", jsonschema.String()),
		jsonschema.Field("accelerator", jsonschema.String()),
		jsonschema.Field("executionMode", jsonschema.String()),
		jsonschema.Required("messages", jsonschema.Array(llmMessageSchema())),
		jsonschema.Field("requireParameters", jsonschema.Boolean()),
		jsonschema.Field("enableResponseHealing", jsonschema.Boolean()),
	).RawMessage()
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
	).RawMessage()
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
		jsonschema.Field("outputDimensions", jsonschema.Integer()),
	).RawMessage()
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
		jsonschema.Field("watchAttemptCount", jsonschema.Integer()),
		jsonschema.Field("lastAttentionAt", jsonschema.String()),
		jsonschema.Field("createdAt", jsonschema.String()),
		jsonschema.Field("updatedAt", jsonschema.String()),
		jsonschema.Field("expiresAt", jsonschema.String()),
		jsonschema.Field("error", jsonschema.String()),
		jsonschema.Field("denialCode", jsonschema.String()),
	).RawMessage()
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
	).RawMessage()
}
