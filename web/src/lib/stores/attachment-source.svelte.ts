import { isSupabaseConfigured, projectURL } from '$lib/supabase';
import { copyAttachmentForReading, type MessengerAttachment } from '$lib/messenger/messenger-api';
import { keptAssetPathOf } from '$lib/messenger/kept-attachment';
import { forgetAttachmentFile, forgetAttachmentFiles, isKeptAsFile, keepAttachmentFile, keptAttachmentFile } from '$lib/messenger/attachment-files';
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
	private openingByURL = $state<Map<string, Promise<void>>>(new Map());
	private notShownURLs = $state<Set<string>>(new Set());
	private asked = new Map<string, MessengerAttachment>();
	private hostCopyByURL = new Map<string, string>();
	private reopenedAfterFailure = new Set<string>();
	private generation = 0;

	constructor() {
		onMessengerCacheReset(() => {
			this.generation += 1;
			this.openableByURL = new Map();
			this.progressByURL = new Map();
			this.failureByURL = new Map();
			this.openingByURL = new Map();
			this.notShownURLs = new Set();
			this.asked.clear();
			this.hostCopyByURL.clear();
			this.reopenedAfterFailure.clear();
			void forgetAttachmentFiles();
		});
	}

	openable(url: string): string {
		return this.openableByURL.get(url)?.address ?? '';
	}

	status(url: string): AttachmentSourceStatus {
		if (this.openableByURL.has(url)) return 'ready';
		if (this.failureByURL.has(url)) return 'failed';
		if (this.openingByURL.has(url)) return 'loading';
		return 'unasked';
	}

	progress(url: string): AttachmentProgress | null {
		return this.progressByURL.get(url) ?? null;
	}

	failure(url: string): string {
		return this.failureByURL.get(url) ?? '';
	}

	isNotShown(url: string): boolean {
		return this.notShownURLs.has(url);
	}

	async readable(url: string, shown: string): Promise<string> {
		const attachment = this.asked.get(url);
		if (!attachment) return shown;
		if (this.isFresh(url)) return this.openable(url);
		await this.open(attachment);
		if (this.isFresh(url)) return this.openable(url);
		throw new Error(this.failure(url) || `the attachment ${url} could not be signed again`);
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
		const shown = this.openableByURL.get(url);
		if (!attachment || !shown || this.openingByURL.has(url)) return;
		if (isStale(shown, Date.now())) {
			void this.open(attachment);
			return;
		}
		if (this.reopenedAfterFailure.has(url)) {
			this.notShownURLs = new Set([...this.notShownURLs, url]);
			return;
		}
		this.reopenedAfterFailure.add(url);
		this.openAgain(attachment);
	}

	retry(url: string): void {
		const attachment = this.asked.get(url);
		if (!attachment || this.openingByURL.has(url)) return;
		this.failureByURL = withoutKey(this.failureByURL, url);
		this.openAgain(attachment);
	}

	private openAgain(attachment: MessengerAttachment): void {
		this.openableByURL = withoutKey(this.openableByURL, attachment.url);
		void this.open(attachment, { withoutKeptCopies: true });
	}

	private isFresh(url: string): boolean {
		const openable = this.openableByURL.get(url);
		return Boolean(openable && !isStale(openable, Date.now()));
	}

	private needsRenewal(url: string, now: number = Date.now()): boolean {
		const openable = this.openableByURL.get(url);
		return Boolean(openable && isStale(openable, now) && !this.openingByURL.has(url));
	}

	private hasFailed(url: string): boolean {
		return this.failureByURL.has(url) && !this.openingByURL.has(url);
	}

	private open(attachment: MessengerAttachment, options: { withoutKeptCopies?: boolean } = {}): Promise<void> {
		const url = attachment.url;
		const inFlight = this.openingByURL.get(url);
		if (inFlight) return inFlight;
		const pending = this.load(attachment, options).finally(() => {
			if (this.openingByURL.get(url) === pending) this.openingByURL = withoutKey(this.openingByURL, url);
		});
		this.openingByURL = new Map(this.openingByURL).set(url, pending);
		return pending;
	}

	private async load(attachment: MessengerAttachment, { withoutKeptCopies = false }: { withoutKeptCopies?: boolean }): Promise<void> {
		const generation = this.generation;
		const url = attachment.url;
		try {
			if (withoutKeptCopies) {
				this.hostCopyByURL.delete(url);
				await forgetAttachmentFile(url);
			}
			const kept = isKeptAsFile(attachment.contentType) ? await keptAttachmentFile(url) : '';
			if (generation !== this.generation) return;
			if (kept) {
				this.opened(url, { address: kept, expiresAt: Number.POSITIVE_INFINITY });
				return;
			}
			const expiresAt = Date.now() + readableForSeconds * 1000;
			const signed = await this.signedCopyOf(attachment, generation);
			if (generation !== this.generation) return;
			this.opened(url, { address: signed, expiresAt });
			if (isKeptAsFile(attachment.contentType)) void keepAttachmentFile(url, signed);
		} catch (failure) {
			if (generation !== this.generation) return;
			const reason = failure instanceof Error ? failure.message : String(failure);
			this.failureByURL = new Map(this.failureByURL).set(url, reason);
		} finally {
			if (generation !== this.generation) return;
			this.progressByURL = withoutKey(this.progressByURL, url);
		}
	}

	private async signedCopyOf(attachment: MessengerAttachment, generation: number): Promise<string> {
		const url = attachment.url;
		if (keptAssetPathOf(projectURL(), url)) return signedForReading(url);
		const remembered = this.hostCopyByURL.get(url);
		if (remembered) {
			const signed = await signedForReading(remembered).catch((failure: unknown) => this.forgetHostCopy(url, failure));
			if (signed) return signed;
		}
		const copy = await this.copied(attachment, generation);
		if (generation !== this.generation) return '';
		this.hostCopyByURL.set(url, copy);
		return signedForReading(copy);
	}

	private forgetHostCopy(url: string, failure: unknown): string {
		console.warn('the store would not sign the copy the company computer made, so it is asked for a new one', { url, failure });
		this.hostCopyByURL.delete(url);
		return '';
	}

	private opened(url: string, openable: Openable): void {
		this.openableByURL = new Map(this.openableByURL).set(url, openable);
		if (this.notShownURLs.has(url)) this.notShownURLs = new Set([...this.notShownURLs].filter((notShown) => notShown !== url));
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
