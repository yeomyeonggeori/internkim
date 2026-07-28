package admind

import "testing"

func TestNormalizeInternationalPhoneNumber(t *testing.T) {
	cases := []struct {
		name        string
		phoneNumber string
		callingCode string
		want        string
	}{
		{name: "empty stays empty", phoneNumber: "  ", callingCode: "82", want: ""},
		{name: "national number takes the workspace calling code", phoneNumber: "010-1234-5678", callingCode: "82", want: "+821012345678"},
		{name: "landline keeps its area code", phoneNumber: "02 123 4567", callingCode: "82", want: "+8221234567"},
		{name: "international prefix becomes plus", phoneNumber: "0082 10 1234 5678", callingCode: "82", want: "+821012345678"},
		{name: "already international is kept", phoneNumber: "+1 (415) 555-0132", callingCode: "82", want: "+14155550132"},
		{name: "missing calling code falls back to the default", phoneNumber: "010-1234-5678", callingCode: "", want: "+821012345678"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			normalized, errorValue := normalizeInternationalPhoneNumber(testCase.phoneNumber, testCase.callingCode)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if normalized != testCase.want {
				t.Fatalf("normalized = %q; want %q", normalized, testCase.want)
			}
		})
	}
}

func TestNormalizeInternationalPhoneNumberRejectsShortNumbers(t *testing.T) {
	if _, errorValue := normalizeInternationalPhoneNumber("1234", "82"); errorValue == nil {
		t.Fatal("expected a short number to be rejected")
	}
	if _, errorValue := normalizeInternationalPhoneNumber("+1234567890123456", "82"); errorValue == nil {
		t.Fatal("expected an over-long number to be rejected")
	}
}
