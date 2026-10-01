const encoder = new TextEncoder();

export async function isTheSameSecret(offered: string, expected: string): Promise<boolean> {
	if (expected === '') return false;
	const comparisonKey = await crypto.subtle.generateKey({ name: 'HMAC', hash: 'SHA-256' }, false, ['sign', 'verify']);
	const expectedSignature = await crypto.subtle.sign('HMAC', comparisonKey, encoder.encode(expected));
	return crypto.subtle.verify('HMAC', comparisonKey, expectedSignature, encoder.encode(offered));
}
