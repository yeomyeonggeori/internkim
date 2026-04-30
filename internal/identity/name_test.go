package identity

import "testing"

func TestSplitNameForMattermost(t *testing.T) {
	testCases := []struct {
		name              string
		expectedFirstName string
		expectedLastName  string
	}{
		{"김민수", "민수", "김"},
		{"이서연", "서연", "이"},
		{"김", "김", ""},
		{"Ada Lovelace", "Ada", "Lovelace"},
		{"Madonna", "Madonna", ""},
		{"Mary Jane Watson", "Mary", "Watson"},
		{"  김민수  ", "민수", "김"},
		{"김 민수", "민수", "김"},
		{"Lee 서연", "Lee", "서연"},
		{"", "", ""},
		{"   ", "", ""},
	}

	for _, testCase := range testCases {
		firstName, lastName := SplitNameForMattermost(testCase.name)
		if firstName != testCase.expectedFirstName || lastName != testCase.expectedLastName {
			t.Errorf("SplitNameForMattermost(%q) = (%q, %q); want (%q, %q)",
				testCase.name, firstName, lastName,
				testCase.expectedFirstName, testCase.expectedLastName)
		}
	}
}

func TestCallingName(t *testing.T) {
	testCases := []struct {
		name     string
		expected string
	}{
		{"김민수", "민수"},
		{"Ada Lovelace", "Ada"},
		{"Madonna", "Madonna"},
		{"", ""},
	}

	for _, testCase := range testCases {
		result := CallingName(testCase.name)
		if result != testCase.expected {
			t.Errorf("CallingName(%q) = %q; want %q", testCase.name, result, testCase.expected)
		}
	}
}
