import { decodeBase64URL, encodeBase64URL } from './base64url.ts';

export function pkcs8FromPEM(pem: string): Uint8Array<ArrayBuffer> {
	const body = pem
		.replace(/-----BEGIN [A-Z ]+-----/, '')
		.replace(/-----END [A-Z ]+-----/, '')
		.replace(/\s+/g, '');
	if (body === '') throw new Error('this key carries no PEM body');
	return decodeBase64URL(body);
}

export async function signedJWT(
	header: Record<string, unknown>,
	payload: Record<string, unknown>,
	key: CryptoKey,
	algorithm: AlgorithmIdentifier | EcdsaParams
): Promise<string> {
	const signed = `${encodeSegment(header)}.${encodeSegment(payload)}`;
	const signature = await crypto.subtle.sign(algorithm, key, new TextEncoder().encode(signed));
	return `${signed}.${encodeBase64URL(signature)}`;
}

function encodeSegment(value: Record<string, unknown>): string {
	return encodeBase64URL(new TextEncoder().encode(JSON.stringify(value)).buffer);
}
