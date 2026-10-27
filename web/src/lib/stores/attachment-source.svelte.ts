import { isSupabaseConfigured, projectURL } from '$lib/supabase';
import { copyAttachmentForReading, type MessengerAttachment } from '$lib/messenger/messenger-api';
import { keptAssetPathOf } from '$lib/messenger/kept-attachment';
import { forgetAttachmentFiles, isKeptAsFile, keepAttachmentFile, keptAttachmentFile } from '$lib/messenger/attachment-files';
import { onMessengerCacheReset } from '$lib/messenger/cache-scope';
import { readableForSeconds, signedForReading } from '$lib/transfer/company-transfer';

export type AttachmentSourceStatus = 'unasked' | 'loading' | 'ready' | 'failed';

export type AttachmentProgress = { copiedBytes: number; totalBytes: number };

type Openable = { address: string; expiresAt: number };

const renewBeforeExpiryMilliseconds = 5 * 60 * 1000;

class AttachmentSourceStore {
	private openableByURL = $state<Map<string, Openable>>(new Map());
	private progressByURL = $state<Map<string, AttachmentProgress>>(new Map());
	private failureByURL = $state<Map<string, string>>(new Map());
	private loadingURLs = $state<Set<string>>(new Set());
	private asked = new Map<string, MessengerAttachment>();
	private generation = 0;

	constructor() {
		onMessengerCacheReset(() => {
			this.generation += 1;
			this.openableByURL = new Map();
			this.progressByURL = new Map();
			this.failureByURL = new Map();
			this.loadingURLs = new Set();
			this.asked.clear();
			void forgetAttachmentFiles();
		});
	}

	openable(url: string): string {
		return this.openableByURL.get(url)?.address ?? '';
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
			if (this.asked.has(attachment.url) && !this.needsRenewal(attachment.url) && !this.hasFailed(attachment.url)) continue;
			this.asked.set(attachment.url, attachment);
			if (this.failureByURL.has(attachment.url)) this.failureByURL = withoutKey(this.failureByURL, attachment.url);
			void this.open(attachment);
		}
	}

	renew(url: string): void {
		const attachment = this.asked.get(url);
		if (!attachment || !this.needsRenewal(url)) return;
		void this.open(attachment);
	}

	retry(url: string): void {
		const attachment = this.asked.get(url);
		if (!attachment || this.loadingURLs.has(url)) return;
		this.failureByURL = withoutKey(this.failureByURL, url);
		void this.open(attachment);
	}

	private needsRenewal(url: string, now: number = Date.now()): boolean {
		const openable = this.openableByURL.get(url);
		return Boolean(openable && isStale(openable, now) && !this.loadingURLs.has(url));
	}

	private hasFailed(url: string): boolean {
		return this.failureByURL.has(url) && !this.loadingURLs.has(url);
	}

	private async open(attachment: MessengerAttachment): Promise<void> {
		const generation = this.generation;
		const url = attachment.url;
		this.loadingURLs = new Set([...this.loadingURLs, url]);
		try {
			const kept = isKeptAsFile(attachment.contentType) ? await keptAttachmentFile(url) : '';
			if (generation !== this.generation) return;
			if (kept) {
				this.openableByURL = new Map(this.openableByURL).set(url, { address: kept, expiresAt: Number.POSITIVE_INFINITY });
				return;
			}
			const address = keptAssetPathOf(projectURL(), url) ? url : await this.copied(attachment, generation);
			if (generation !== this.generation) return;
			const expiresAt = Date.now() + readableForSeconds * 1000;
			const signed = await signedForReading(address);
			if (generation !== this.generation) return;
			this.openableByURL = new Map(this.openableByURL).set(url, { address: signed, expiresAt });
			if (isKeptAsFile(attachment.contentType)) void keepAttachmentFile(url, signed);
		} catch (failure) {
			if (generation !== this.generation) return;
			const reason = failure instanceof Error ? failure.message : String(failure);
			this.failureByURL = new Map(this.failureByURL).set(url, reason);
		} finally {
			if (generation !== this.generation) return;
			this.progressByURL = withoutKey(this.progressByURL, url);
			this.loadingURLs = new Set([...this.loadingURLs].filter((loading) => loading !== url));
		}
	}

	private async copied(attachment: MessengerAttachment, generation: number): Promise<string> {
		const copy = await copyAttachmentForReading(attachment, (copiedBytes, totalBytes) => {
			if (generation !== this.generation) return;
			this.progressByURL = new Map(this.progressByURL).set(attachment.url, { copiedBytes, totalBytes });
		});
		return copy.address;
	}
}

function isStale(openable: Openable, now: number): boolean {
	return openable.expiresAt - renewBeforeExpiryMilliseconds <= now;
}

function withoutKey<Value>(values: Map<string, Value>, key: string): Map<string, Value> {
	const remaining = new Map(values);
	remaining.delete(key);
	return remaining;
}

export const attachmentSource = new AttachmentSourceStore();
