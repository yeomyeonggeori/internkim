export function hasCRMContactMethod(email: string, phone: string): boolean {
	return email.trim() !== '' || phone.trim() !== '';
}
