import { isSupabaseConfigured, projectURL, supabase } from '$lib/supabase';
import { keepAttachmentForReading, type MessengerAttachment } from '$lib/messenger/messenger-api';
import { addressesToSign, assetBucket, readableAddresses } from '$lib/messenger/kept-attachment';

class AttachmentSourceStore {
	private openableByURL = $state<Map<string, string>>(new Map());
	private asked = new Set<string>();

	openable(url: string): string {
		return this.openableByURL.get(url) ?? '';
	}

	async wants(attachments: MessengerAttachment[]): Promise<void> {
		if (!isSupabaseConfigured()) return;
		const wanted = attachments.filter((attachment) => !this.asked.has(attachment.url));
		if (wanted.length === 0) return;
		wanted.forEach((attachment) => this.asked.add(attachment.url));

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
	}
}

export const attachmentSource = new AttachmentSourceStore();
