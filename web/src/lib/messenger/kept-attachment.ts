const assetBucket = 'asset';
const objectPrefix = `/storage/v1/object/${assetBucket}/`;
const readableForSeconds = 60 * 60 * 24;

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

// A file the messenger's own store would not take is kept in the company's
// bucket instead, and the message names the object rather than a link that
// outlives it. Row level security decides who may read it, so the reader signs
// for it here with their own session.
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
