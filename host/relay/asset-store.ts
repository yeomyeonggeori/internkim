export type AssetUploader = {
	upload: (
		path: string,
		body: Uint8Array,
		options: { contentType: string; upsert: boolean }
	) => Promise<{ error: { message: string } | null }>;
	createSignedUrl: (
		path: string,
		expiresIn: number
	) => Promise<{ data: { signedUrl: string } | null; error: { message: string } | null }>;
};

export const assetBucket = 'asset';
const signedURLSeconds = 60 * 60 * 24;

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

export async function digestOf(bytes: Uint8Array): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', bytes as BufferSource);
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

export function sharedAssetPath(companyID: string, kind: string, digest: string, contentType: string): string {
	return `${companyID}/shared/${kind}/${digest}${extensionOf(contentType)}`;
}

function isAlreadyStored(refusal: string): boolean {
	return refusal.toLowerCase().includes('already exists') || refusal.includes('409');
}

export async function keepSharedAsset(
	uploader: AssetUploader,
	companyID: string,
	kind: string,
	bytes: Uint8Array,
	contentType: string
): Promise<string> {
	const path = sharedAssetPath(companyID, kind, await digestOf(bytes), contentType);
	const written = await uploader.upload(path, bytes, { contentType, upsert: false });
	if (written.error && !isAlreadyStored(written.error.message)) {
		throw new Error(`the asset store refused ${path}: ${written.error.message}`);
	}
	const signed = await uploader.createSignedUrl(path, signedURLSeconds);
	if (signed.error || !signed.data) {
		throw new Error(`the asset store would not sign ${path}: ${signed.error?.message ?? 'no url'}`);
	}
	return signed.data.signedUrl;
}
