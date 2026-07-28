import { finalizeEvent } from 'nostr-tools/pure';

export type BlossomBlob = {
	url: string;
	sha256: string;
	size: number;
	mimeType: string;
};

export function blossomBaseURL(relayURL: string): string {
	if (relayURL.startsWith('wss://')) return 'https://' + relayURL.slice('wss://'.length);
	if (relayURL.startsWith('ws://')) return 'http://' + relayURL.slice('ws://'.length);
	return relayURL;
}

export function imetaTag(blob: BlossomBlob): string[] {
	return ['imeta', 'url ' + blob.url, 'm ' + blob.mimeType, 'x ' + blob.sha256, 'size ' + String(blob.size)];
}

function hexToBytes(hex: string): Uint8Array {
	const bytes = new Uint8Array(hex.length / 2);
	for (let index = 0; index < bytes.length; index++) {
		bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
	}
	return bytes;
}

async function sha256Hex(content: Uint8Array): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', content as BufferSource);
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

// Uploads a blob to the relay's Blossom store, authorized by a kind-24242 event
// the person signs with their own key in the browser — so attachments, like
// messages, are authored client-side and never handled server-signed.
export async function uploadBlob(
	relayURL: string,
	secretHex: string,
	content: Uint8Array,
	mimeType: string
): Promise<BlossomBlob> {
	const digestHex = await sha256Hex(content);
	const nowSeconds = Math.floor(Date.now() / 1000);
	const authEvent = finalizeEvent(
		{
			kind: 24242,
			content: 'upload',
			created_at: nowSeconds,
			tags: [
				['t', 'upload'],
				['x', digestHex],
				['expiration', String(nowSeconds + 3600)]
			]
		},
		hexToBytes(secretHex)
	);
	const response = await fetch(blossomBaseURL(relayURL) + '/upload', {
		method: 'PUT',
		headers: {
			Authorization: 'Nostr ' + btoa(JSON.stringify(authEvent)),
			'Content-Type': mimeType,
			'X-SHA-256': digestHex
		},
		body: new Blob([content as BlobPart], { type: mimeType })
	});
	if (!response.ok) {
		throw new Error(`blossom upload returned ${response.status}: ${(await response.text()).trim()}`);
	}
	const record = (await response.json()) as Record<string, unknown>;
	return {
		url: typeof record.url === 'string' ? record.url : '',
		sha256: typeof record.sha256 === 'string' ? record.sha256 : digestHex,
		size: typeof record.size === 'number' ? record.size : content.byteLength,
		mimeType: typeof record.type === 'string' ? record.type : mimeType
	};
}
