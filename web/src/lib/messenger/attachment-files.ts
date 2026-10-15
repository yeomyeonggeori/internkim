const cacheName = 'messenger-attachment-files';

const shownURLOfAddress = new Map<string, string>();

export function isKeptAsFile(contentType: string): boolean {
	return contentType.startsWith('image/');
}

export async function keptAttachmentFile(address: string): Promise<string> {
	const known = shownURLOfAddress.get(address);
	if (known) return known;
	try {
		const cache = await openAttachmentFiles();
		const kept = await cache?.match(fileKeyOf(address));
		return kept ? shownURLOf(address, await kept.blob()) : '';
	} catch (failure) {
		console.warn('a messenger attachment kept on this device could not be read', { address, failure });
		return '';
	}
}

export async function keepAttachmentFile(address: string, readableURL: string): Promise<void> {
	try {
		const cache = await openAttachmentFiles();
		if (!cache) return;
		const response = await fetch(readableURL);
		if (!response.ok) throw new Error(`the attachment at ${address} answered ${response.status}`);
		const file = await response.blob();
		if (!isKeptAsFile(file.type)) throw new Error(`the attachment at ${address} is ${file.type}, not an image`);
		await cache.put(fileKeyOf(address), new Response(file, { headers: { 'Content-Type': file.type } }));
	} catch (failure) {
		console.warn('a messenger attachment could not be kept on this device', { address, failure });
	}
}

export async function forgetAttachmentFiles(): Promise<void> {
	for (const shown of shownURLOfAddress.values()) URL.revokeObjectURL(shown);
	shownURLOfAddress.clear();
	try {
		if (typeof caches !== 'undefined') await caches.delete(cacheName);
	} catch (failure) {
		console.warn('the messenger could not forget the attachments it kept on this device', failure);
	}
}

async function openAttachmentFiles(): Promise<Cache | null> {
	if (typeof caches === 'undefined' || typeof location === 'undefined') return null;
	return caches.open(cacheName);
}

function fileKeyOf(address: string): string {
	return new URL(`/messenger-attachment-file/${encodeURIComponent(address)}`, location.origin).href;
}

function shownURLOf(address: string, file: Blob): string {
	const shown = URL.createObjectURL(file);
	shownURLOfAddress.set(address, shown);
	return shown;
}
