export const assetBucket = 'asset';
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

export type AttachmentToOpen = { url: string };

// A file the messenger refused at send time is already an object in the
// company's bucket. Everything else is addressed on the company's own machine,
// which this browser cannot reach, so the relay is asked to put a copy where it
// can. Either way what comes back is an object nobody may read unsigned.
export async function addressesToSign<Attachment extends AttachmentToOpen>(
	attachments: Attachment[],
	projectURL: string,
	keep: (attachment: Attachment) => Promise<{ address: string } | null>
): Promise<Map<string, string>> {
	const found = await Promise.all(
		attachments.map(async (attachment) => {
			if (keptAssetPathOf(projectURL, attachment.url)) return [attachment.url, attachment.url] as const;
			const kept = await keep(attachment).catch(() => null);
			return [attachment.url, kept?.address ?? ''] as const;
		})
	);
	return new Map(found.filter(([, address]) => address !== ''));
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
