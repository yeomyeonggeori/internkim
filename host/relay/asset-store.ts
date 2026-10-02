export type AssetUploader = {
	upload: (
		path: string,
		body: Uint8Array,
		options: { contentType: string; upsert: boolean }
	) => Promise<{ error: { message: string } | null }>;
};

export const assetBucket = 'asset';

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

export const attachmentKind = 'attachment';
export const personPictureKind = 'person-picture';

function isAlreadyStored(refusal: string): boolean {
	return refusal.toLowerCase().includes('already exists') || refusal.includes('409');
}

// A file the relay would not take still belongs to the conversation it was sent
// to, so it is kept where the company can read it and named by what it is. The
// address is the object's own, not a signed one: whoever reads it signs for
// themselves with their own session, so nothing long-lived is written into a
// message that cannot be edited afterwards.
export function attachmentAddress(projectURL: string, path: string): string {
	return `${projectURL.replace(/\/+$/, '')}/storage/v1/object/${assetBucket}/${path}`;
}

export type AssetLister = {
	list: (
		path: string,
		options: { search: string; limit: number }
	) => Promise<{ data: { name: string; metadata?: { size?: number } | null }[] | null; error: unknown }>;
};

// A picture is named by its bytes alone: the extension it was kept under is
// whatever type the messenger declared, which the name it is asked for by
// does not carry.
export async function sharedAssetKeptAs(
	lister: AssetLister,
	companyID: string,
	kind: string,
	digest: string
): Promise<string | null> {
	const directory = `${companyID}/shared/${kind}`;
	const listed = await lister.list(directory, { search: digest, limit: 5 });
	const found = listed.data?.find((one) => one.name === digest || one.name.startsWith(`${digest}.`));
	return found ? `${directory}/${found.name}` : null;
}

export async function keepSharedAsset(
	uploader: AssetUploader,
	companyID: string,
	kind: string,
	bytes: Uint8Array,
	contentType: string
): Promise<{ path: string; digest: string }> {
	const digest = await digestOf(bytes);
	const path = sharedAssetPath(companyID, kind, digest, contentType);
	const written = await uploader.upload(path, bytes, { contentType, upsert: false });
	if (written.error && !isAlreadyStored(written.error.message)) {
		// The size and the type are what a refusal usually turns on, and neither is
		// in the store's own message. A reader left with "refused" has to guess.
		throw new Error(
			`the asset store refused ${path} (${bytes.byteLength} bytes, ${contentType || 'no content type'}): ${written.error.message}`
		);
	}
	return { path, digest };
}
