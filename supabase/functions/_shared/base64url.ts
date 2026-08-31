export function decodeBase64URL(encoded: string): Uint8Array<ArrayBuffer> {
	const padded = encoded.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(encoded.length / 4) * 4, '=');
	const binary = atob(padded);
	return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

export function encodeBase64URL(bytes: ArrayBufferLike | null): string {
	if (!bytes) return '';
	const binary = String.fromCharCode(...new Uint8Array(bytes));
	return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}
