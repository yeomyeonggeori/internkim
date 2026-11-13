package capabilityprotocol

var browserToolNames = []string{
	"browser_open",
	"browser_snapshot",
	"browser_screenshot",
	"browser_click",
	"browser_fill",
	"browser_select",
	"browser_press",
	"browser_wait",
	"browser_handoff",
}

func BrowserToolDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(browserToolNames...))
}

var desktopToolNames = []string{
	"desktop_read",
	"desktop_act",
}

func DesktopToolDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors(MustGeneratedToolDescriptors(desktopToolNames...))
}
