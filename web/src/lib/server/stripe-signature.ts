const encoder = new TextEncoder();
const toleranceSeconds = 300;

export async function isSignedByStripe(
	payload: string,
	signatureHeader: string,
	signingSecret: string,
	nowSeconds: number = Math.floor(Date.now() / 1000)
): Promise<boolean> {
	if (signingSecret === '') return false;
	const elements = signatureHeader.split(',').map((element) => element.split('='));
	const timestamp = Number(elements.find(([prefix]) => prefix === 't')?.[1]);
	const signatures = elements.filter(([prefix]) => prefix === 'v1').map(([, value]) => value ?? '');
	if (!Number.isInteger(timestamp) || signatures.length === 0) return false;
	if (Math.abs(nowSeconds - timestamp) > toleranceSeconds) return false;

	const key = await crypto.subtle.importKey('raw', encoder.encode(signingSecret), { name: 'HMAC', hash: 'SHA-256' }, false, [
		'verify'
	]);
	const signedPayload = encoder.encode(`${timestamp}.${payload}`);
	for (const signature of signatures) {
		const signatureBytes = bytesOfHex(signature);
		if (signatureBytes && (await crypto.subtle.verify('HMAC', key, signatureBytes, signedPayload))) return true;
	}
	return false;
}

function bytesOfHex(hex: string): Uint8Array<ArrayBuffer> | null {
	if (!/^[0-9a-f]+$/.test(hex) || hex.length % 2 !== 0) return null;
	const bytes = new Uint8Array(new ArrayBuffer(hex.length / 2));
	for (let index = 0; index < bytes.length; index += 1) {
		bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
	}
	return bytes;
}
