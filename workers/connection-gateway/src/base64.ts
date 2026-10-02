export function base64Of(bytes: Uint8Array): string {
	let binary = '';
	for (let index = 0; index < bytes.byteLength; index += 0x8000) {
		binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000));
	}
	return btoa(binary);
}

export function bytesOfBase64(base64: string): Uint8Array<ArrayBuffer> {
	return Uint8Array.from(atob(base64), (character) => character.charCodeAt(0));
}
