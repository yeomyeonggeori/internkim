package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

// Where a schedule tool is answered, and whether the schedule it writes has to
// carry the conversation this turn is happening in. That binding is a fact the
// runtime holds, so the model is never asked for it.
type scheduleToolUpstream struct {
	admindPath    string
	bindsDelivery bool
}

var scheduleToolUpstreams = map[string]scheduleToolUpstream{
	"schedule_list":   {admindPath: "/memory/api/schedules/tool-list"},
	"schedule_create": {admindPath: "/memory/api/schedules/tool-create", bindsDelivery: true},
	"schedule_update": {admindPath: "/memory/api/schedules/tool-update"},
	"schedule_cancel": {admindPath: "/memory/api/schedules/tool-cancel"},
}

func (service Service) invokeScheduleTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	upstream, isRouted := scheduleToolUpstreams[request.ToolName]
	if !isRouted {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("schedule tool is not configured: %s", request.ToolName)
	}
	body, errorValue := scheduleToolBody(request, upstream)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerThroughAdmindAsTheRequester(ctx, request, upstream.admindPath, body)
}

// A schedule answers where it was asked for. A call arriving outside any
// conversation carries nothing to bind to, so the fields stay absent and the
// record refuses the schedule rather than delivering its runs somewhere nobody
// chose.
func scheduleToolBody(request capabilities.ToolInvokeRequest, upstream scheduleToolUpstream) ([]byte, error) {
	input := request.Input
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("{}")
	}
	if !upstream.bindsDelivery {
		return input, nil
	}
	document := map[string]json.RawMessage{}
	if errorValue := json.Unmarshal(input, &document); errorValue != nil {
		return nil, errorValue
	}
	for field, value := range scheduleDeliveryBinding(request.Context) {
		encoded, errorValue := json.Marshal(value)
		if errorValue != nil {
			return nil, errorValue
		}
		document[field] = encoded
	}
	return json.Marshal(document)
}

func scheduleDeliveryBinding(toolContext capabilities.ToolInvokeContext) map[string]string {
	binding := map[string]string{}
	for field, value := range map[string]string{
		"platform":       toolContext.Platform,
		"conversationID": toolContext.ConversationID,
		"replyTargetID":  toolContext.ReplyTargetID,
	} {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		binding[field] = trimmedValue
	}
	return binding
}
