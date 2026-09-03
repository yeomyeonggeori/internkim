package admind

func policyString(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}
