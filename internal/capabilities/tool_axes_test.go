package capabilities

import "testing"

// Routing and approval read these axes off the descriptor.
// A tool that leaves one blank silently falls out of the behaviour it belongs to,
// which is exactly what name matching used to hide.

func TestEveryBrowserToolDeclaresTheAxesItsCallersRead(t *testing.T) {
	for _, descriptor := range BrowserToolDescriptors() {
		if descriptor.Namespace != "browser" {
			t.Errorf("%s is a browser tool outside the browser namespace", descriptor.Name)
		}
		if descriptor.ApprovalScope == "" {
			t.Errorf("%s needs a grant but declares no approval scope", descriptor.Name)
		}
	}
}

func TestNoToolIsNamedWithADot(t *testing.T) {
	for _, descriptor := range append(BrowserToolDescriptors(), DefaultToolDescriptors()...) {
		for _, name := range []string{descriptor.Name, descriptor.CanonicalName, descriptor.ModelName} {
			for _, character := range name {
				if character == '.' {
					t.Errorf("tool name %q is not a valid variable-style name", name)
					break
				}
			}
		}
	}
}
