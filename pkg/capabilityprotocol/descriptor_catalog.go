package capabilityprotocol

const (
	BrowserApprovalScope   = "browser"
	FileApprovalScope      = "file"
	UserInputApprovalScope = "user_input"
)

var browserToolNames = []string{
	"browser_open",
	"browser_snapshot",
	"browser_screenshot",
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

func BrowserToolDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(browserToolNames...))
}

func DeviceBrowserDescriptors() []Descriptor {
	descriptors := []Descriptor{}
	for _, descriptor := range MustGeneratedToolDescriptors(browserToolNames...) {
		description, isDeviceBrowserTool := deviceBrowserDescriptions[descriptor.Name]
		if !isDeviceBrowserTool {
			continue
		}
		descriptor.Description = description
		descriptor.PrivacyClass = "device_browser"
		descriptor.RequiresUserPresence = false
		if !deviceBrowserToolsUnderTheBrowserGrant[descriptor.Name] {
			descriptor.RequiresRequesterDevice = false
			descriptor.ApprovalScope = ""
		}
		descriptors = append(descriptors, descriptor)
	}
	return MustCanonicalizeBuiltInDescriptors(descriptors)
}
