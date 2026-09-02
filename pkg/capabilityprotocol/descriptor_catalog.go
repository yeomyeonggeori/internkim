package capabilityprotocol

// A grant covers a family of tools, and each tool names the family it belongs to.
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
	"browser_handoff",
	"browser_click",
	"browser_fill",
	"browser_select",
	"browser_press",
	"browser_wait",
}

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
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(companionBrowserToolNames...))
}

func CompanionLLMDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(
		"llm_text",
		"llm_structured",
		"embedding_create",
		AttentionTriageToolName,
	))
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
