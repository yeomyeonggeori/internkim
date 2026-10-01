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

func BrowserToolDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(browserToolNames...))
}
