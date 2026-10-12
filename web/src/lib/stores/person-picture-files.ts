const cacheName = 'person-picture-files';

const shownURLOfAddress = new Map<string, string>();

export async function keptPictureFiles(addresses: string[]): Promise<Map<string, string>> {
	const cache = await openPictureFiles();
	const shown = new Map<string, string>();
	if (!cache) return shown;
	await Promise.all(
		addresses.map(async (address) => {
			const known = shownURLOfAddress.get(address);
			if (known) return shown.set(address, known);
			const kept = await cache.match(fileKeyOf(address));
			if (!kept) return;
			shown.set(address, shownURLOf(address, await kept.blob()));
		})
	);
	return shown;
}

export async function keepPictureFiles(readableOfAddress: Map<string, string>): Promise<Map<string, string>> {
	const shown = new Map<string, string>();
	await Promise.all(
		[...readableOfAddress].map(async ([address, readableURL]) => {
			try {
				const file = await keepPictureFile(address, readableURL);
				if (file) shown.set(address, file);
			} catch (failure) {
				console.warn('a person picture could not be kept on this device', { address, failure });
			}
		})
	);
	return shown;
}

async function keepPictureFile(address: string, readableURL: string): Promise<string> {
	const known = shownURLOfAddress.get(address);
	if (known) return known;
	const cache = await openPictureFiles();
	if (!cache) return '';
	const response = await fetch(readableURL);
	if (!response.ok) throw new Error(`the picture at ${address} answered ${response.status}`);
	const picture = await response.blob();
	if (!picture.type.startsWith('image/')) throw new Error(`the picture at ${address} is ${picture.type}, not an image`);
	await cache.put(fileKeyOf(address), new Response(picture, { headers: { 'Content-Type': picture.type } }));
	return shownURLOf(address, picture);
}

export async function forgetPictureFiles(): Promise<void> {
	for (const shown of shownURLOfAddress.values()) URL.revokeObjectURL(shown);
	shownURLOfAddress.clear();
	if (typeof caches === 'undefined') return;
	await caches.delete(cacheName);
}

async function openPictureFiles(): Promise<Cache | null> {
	if (typeof caches === 'undefined' || typeof location === 'undefined') return null;
	return caches.open(cacheName);
}

function fileKeyOf(address: string): string {
	return new URL(`/person-picture-file/${encodeURIComponent(address)}`, location.origin).href;
}

function shownURLOf(address: string, picture: Blob): string {
	const shown = URL.createObjectURL(picture);
	shownURLOfAddress.set(address, shown);
	return shown;
}
