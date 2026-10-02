export const assetBucket = 'asset';
const objectPrefix = `/storage/v1/object/${assetBucket}/`;
export const readableForSeconds = 60 * 60 * 24;

export type AttachmentSigner = {
	createSignedUrls: (
		paths: string[],
		expiresIn: number
	) => Promise<{
		data: { path: string | null; signedUrl: string | null }[] | null;
		error: { message: string } | null;
	}>;
};

export function keptAssetPathOf(projectURL: string, address: string): string | null {
	if (!projectURL) return null;
	const prefix = `${projectURL.replace(/\/+$/, '')}${objectPrefix}`;
	return address.startsWith(prefix) ? address.slice(prefix.length) : null;
}

// Row level security decides who may read an object in the company's bucket, so
// the reader signs for it here with their own session. The address in the
// message is the object's own: a message cannot be edited when a link inside it
// expires.
export async function readableAddresses(
	signer: AttachmentSigner,
	projectURL: string,
	addresses: string[]
): Promise<Map<string, string>> {
	const paths = new Map<string, string>();
	for (const address of addresses) {
		const path = keptAssetPathOf(projectURL, address);
		if (path) paths.set(path, address);
	}
	if (paths.size === 0) return new Map();

	const signed = await signer.createSignedUrls([...paths.keys()], readableForSeconds);
	if (signed.error || !signed.data) return new Map();
	const readable = new Map<string, string>();
	for (const one of signed.data) {
		const address = one.path === null ? undefined : paths.get(one.path);
		if (address && one.signedUrl) readable.set(address, one.signedUrl);
	}
	return readable;
}
