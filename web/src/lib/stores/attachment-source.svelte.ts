import { isSupabaseConfigured, projectURL } from '$lib/supabase';
import { copyAttachmentForReading, type MessengerAttachment } from '$lib/messenger/messenger-api';
import { keptAssetPathOf } from '$lib/messenger/kept-attachment';
import { signedForReading } from '$lib/transfer/company-transfer';

export type AttachmentSourceStatus = 'unasked' | 'loading' | 'ready' | 'failed';

export type AttachmentProgress = { copiedBytes: number; totalBytes: number };

class AttachmentSourceStore {
	private openableByURL = $state<Map<string, string>>(new Map());
	private progressByURL = $state<Map<string, AttachmentProgress>>(new Map());
	private failureByURL = $state<Map<string, string>>(new Map());
	private loadingURLs = $state<Set<string>>(new Set());
	private asked = new Map<string, MessengerAttachment>();

	openable(url: string): string {
		return this.openableByURL.get(url) ?? '';
	}

	status(url: string): AttachmentSourceStatus {
		if (this.openableByURL.has(url)) return 'ready';
		if (this.failureByURL.has(url)) return 'failed';
		if (this.loadingURLs.has(url)) return 'loading';
		return 'unasked';
	}

	progress(url: string): AttachmentProgress | null {
		return this.progressByURL.get(url) ?? null;
	}

	failure(url: string): string {
		return this.failureByURL.get(url) ?? '';
	}

	wants(attachments: MessengerAttachment[]): void {
		if (!isSupabaseConfigured()) return;
		for (const attachment of attachments) {
			if (this.asked.has(attachment.url)) continue;
			this.asked.set(attachment.url, attachment);
			void this.open(attachment);
		}
	}

	retry(url: string): void {
		const attachment = this.asked.get(url);
		if (!attachment || this.loadingURLs.has(url)) return;
		this.failureByURL = withoutKey(this.failureByURL, url);
		void this.open(attachment);
	}

	private async open(attachment: MessengerAttachment): Promise<void> {
		const url = attachment.url;
		this.loadingURLs = new Set([...this.loadingURLs, url]);
		try {
			const address = keptAssetPathOf(projectURL(), url) ? url : await this.copied(attachment);
			this.openableByURL = new Map(this.openableByURL).set(url, await signedForReading(address));
		} catch (failure) {
			const reason = failure instanceof Error ? failure.message : String(failure);
			this.failureByURL = new Map(this.failureByURL).set(url, reason);
		} finally {
			this.progressByURL = withoutKey(this.progressByURL, url);
			this.loadingURLs = new Set([...this.loadingURLs].filter((loading) => loading !== url));
		}
	}

	private async copied(attachment: MessengerAttachment): Promise<string> {
		const copy = await copyAttachmentForReading(attachment, (copiedBytes, totalBytes) => {
			this.progressByURL = new Map(this.progressByURL).set(attachment.url, { copiedBytes, totalBytes });
		});
		return copy.address;
	}
}

function withoutKey<Value>(values: Map<string, Value>, key: string): Map<string, Value> {
	const remaining = new Map(values);
	remaining.delete(key);
	return remaining;
}

export const attachmentSource = new AttachmentSourceStore();
