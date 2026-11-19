const transferIdentifierBytes = 16;

export function newTransferIdentifier(): string {
	const randomBytes = crypto.getRandomValues(new Uint8Array(transferIdentifierBytes));
	return Array.from(randomBytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
}
