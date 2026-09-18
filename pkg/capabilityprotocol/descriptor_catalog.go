package capabilityprotocol

import "encoding/json"

const (
	BrowserApprovalScope   = "browser"
	FileApprovalScope      = "file"
	DesktopApprovalScope   = "desktop"
	UserInputApprovalScope = "user_input"
)

var companionBrowserToolNames = []string{
	"browser_open",
	"browser_snapshot",
	"browser_screenshot",
	"browser_click",
	"browser_fill",
	"browser_select",
	"browser_press",
	"browser_wait",
}

const ComputerTaskToolName = "computer_task"

var companionToolNames = append(append([]string{}, companionBrowserToolNames...), ComputerTaskToolName)

var deviceBrowserDescriptions = map[string]string{
	"browser_open":     "Open an exact HTTP or HTTPS URL in the device browser.",
	"browser_snapshot": "Read the current device browser page structure.",
	"browser_click":    "Click one exact target from the current device browser snapshot.",
	"browser_fill":     "Fill text into a target in the device browser.",
	"browser_select":   "Select a value in the device browser.",
	"browser_press":    "Press a key in the device browser.",
	"browser_wait":     "Wait for a browser target or a bounded interval.",
}

var deviceBrowserToolsUnderTheBrowserGrant = map[string]bool{
	"browser_fill":   true,
	"browser_select": true,
	"browser_press":  true,
	"browser_wait":   true,
}

func CompanionToolDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(companionToolNames...))
}

func CompanionLLMDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors([]Descriptor{
		companionJobDescriptor(
			"llm_text",
			"llm",
			"Generate text with whichever language model the runtime routes to.",
			TextLLMInputSchema(),
		),
		companionJobDescriptor(
			"llm_structured",
			"llm",
			"Generate schema-constrained output with whichever language model the runtime routes to.",
			StructuredLLMInputSchema(),
		),
		companionJobDescriptor(
			"embedding_create",
			"embedding",
			"Create embeddings with whichever embedding model the runtime routes to.",
			EmbeddingInputSchema(),
		),
		companionJobDescriptor(
			AttentionTriageToolName,
			"attention",
			"Classify whether a pending companion job needs remote attention.",
			attentionTriageInputSchema(),
		),
	})
}

func companionJobDescriptor(name string, namespace string, description string, inputSchema json.RawMessage) Descriptor {
	return NewDescriptor(DescriptorDefinition{
		Identity: DescriptorIdentity{
			Name:            name,
			CanonicalName:   name,
			Namespace:       namespace,
			AnsweredBy:      AnsweredByLocal,
			ModelName:       name,
			ModelVisibility: ModelVisibilityHidden,
		},
		Metadata: DescriptorMetadata{
			Description:      description,
			Version:          "1",
			PrivacyClass:     "model_input",
			EstimatedLatency: "low",
			WorksOffline:     true,
			InputSchema:      inputSchema,
			OutputSchema:     ToolInvokeOutputSchema(),
			PolicyResource:   "tool:" + name,
			SideEffect:       SideEffectComputation,
			Availability:     AvailabilityMetadata{State: AvailabilityOK},
			Idempotency:      IdempotencyMetadata{Scope: "operation"},
		},
	})
}

func DeviceBrowserDescriptors() []Descriptor {
	descriptors := []Descriptor{}
	for _, descriptor := range MustGeneratedToolDescriptors(companionBrowserToolNames...) {
		description, isDeviceBrowserTool := deviceBrowserDescriptions[descriptor.Name]
		if !isDeviceBrowserTool {
			continue
		}
		descriptor.Description = description
		descriptor.PrivacyClass = "device_browser"
		descriptor.RequiresUserPresence = false
		descriptor.RequiresCompanionBrowser = false
		if !deviceBrowserToolsUnderTheBrowserGrant[descriptor.Name] {
			descriptor.RequiresRequesterDevice = false
			descriptor.ApprovalScope = ""
		}
		descriptors = append(descriptors, descriptor)
	}
	return MustCanonicalizeBuiltInDescriptors(descriptors)
}
