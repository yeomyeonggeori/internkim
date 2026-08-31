export const assetBucket = 'asset';
export const attachmentKind = 'attachment';

const extensions: Record<string, string> = {
	'image/png': '.png',
	'image/jpeg': '.jpg',
	'image/gif': '.gif',
	'image/webp': '.webp',
	'image/svg+xml': '.svg'
};

export function extensionOf(contentType: string): string {
	return extensions[contentType.split(';')[0].trim().toLowerCase()] ?? '';
}

export async function digestOf(bytes: Uint8Array<ArrayBuffer>): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', bytes);
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

export function sharedAssetPath(companyID: string, kind: string, digest: string, contentType: string): string {
	return `${companyID}/shared/${kind}/${digest}${extensionOf(contentType)}`;
}
