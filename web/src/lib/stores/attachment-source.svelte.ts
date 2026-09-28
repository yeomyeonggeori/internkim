import { isSupabaseConfigured, projectURL, supabase } from '$lib/supabase';
import { keepAttachmentForReading, type MessengerAttachment } from '$lib/messenger/messenger-api';
import { addressesToSign, assetBucket, readableAddresses } from '$lib/messenger/kept-attachment';

export type AttachmentSourceStatus = 'unasked' | 'loading' | 'ready' | 'failed';

class AttachmentSourceStore {
	private openableByURL = $state<Map<string, string>>(new Map());
	private loadingURLs = $state<Set<string>>(new Set());
	private failedURLs = $state<Set<string>>(new Set());
	private asked = new Set<string>();

	openable(url: string): string {
		return this.openableByURL.get(url) ?? '';
	}

	status(url: string): AttachmentSourceStatus {
		if (this.openableByURL.has(url)) return 'ready';
		if (this.failedURLs.has(url)) return 'failed';
		if (this.loadingURLs.has(url)) return 'loading';
		return 'unasked';
	}

	async wants(attachments: MessengerAttachment[]): Promise<void> {
		if (!isSupabaseConfigured()) return;
		const wanted = attachments.filter((attachment) => !this.asked.has(attachment.url));
		if (wanted.length === 0) return;
		const wantedURLs = wanted.map((attachment) => attachment.url);
		wantedURLs.forEach((url) => this.asked.add(url));
		this.loadingURLs = new Set([...this.loadingURLs, ...wantedURLs]);

		try {
			const addresses = await addressesToSign(wanted, projectURL(), keepAttachmentForReading);
			const readable = await readableAddresses(
				supabase().storage.from(assetBucket),
				projectURL(),
				[...addresses.values()]
			);
			const next = new Map(this.openableByURL);
			for (const [url, address] of addresses) {
				const signed = readable.get(address);
				if (signed) next.set(url, signed);
			}
			this.openableByURL = next;
		} finally {
			const unopened = wantedURLs.filter((url) => !this.openableByURL.has(url));
			this.failedURLs = new Set([...this.failedURLs, ...unopened]);
			this.loadingURLs = new Set([...this.loadingURLs].filter((url) => !wantedURLs.includes(url)));
		}
	}
}

export const attachmentSource = new AttachmentSourceStore();
