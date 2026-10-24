import { attachmentAddress } from './asset-store';
import { reasonOf } from './failure';
import {
	copyIntoStore,
	moveWithinStore,
	signedReadURL,
	TransferFailed,
	type CopyProgress,
	type RangedSource,
	type StoreAccess
} from './transfer-store';
import type { Transfers, TransferWatcher } from './transfers';

export const copyAddressReadableSeconds = 60;

export type MediaReaderOf = (requesterKey: string, mediaURL: string) => Promise<RangedSource | null>;

export type MessengerCopySettings = {
	companyID: string;
	access: StoreAccess;
	transfers: Transfers;
	readerOf: MediaReaderOf;
	report: (line: string) => void;
};

export type MessengerCopy = { digest: string; contentType: string; source: RangedSource };

export type RequestedCopy = { digest: string; contentType: string; requesterKey: string; mediaURL: string };

export type FilledCopy = { address: string; sizeBytes: number };

export class MessengerCopies {
	constructor(private readonly settings: MessengerCopySettings) {}

	pathOf(digest: string): string {
		return `${this.settings.companyID}/shared/transfer/${digest}`;
	}

	async readableAddressOf(digest: string): Promise<string | null> {
		try {
			return await signedReadURL(this.settings.access, this.pathOf(digest), copyAddressReadableSeconds);
		} catch (failure) {
			if (failure instanceof TransferFailed && failure.status === 404) return null;
			throw failure;
		}
	}

	fill(copy: MessengerCopy, watcher: TransferWatcher | null): void {
		this.settings.transfers.start(this.pathOf(copy.digest), watcher, (progress) => this.filled(copy.digest, () => copy, progress));
	}

	fillAsRequester(requested: RequestedCopy): void {
		this.settings.transfers.start(this.pathOf(requested.digest), null, (progress) =>
			this.filled(requested.digest, () => this.copyReadAsRequester(requested), progress)
		);
	}

	async keepUpload(stagedPath: string, digest: string): Promise<boolean> {
		try {
			await moveWithinStore(this.settings.access, stagedPath, this.pathOf(digest));
			return true;
		} catch (failure) {
			this.reportUnfilled(digest, `the upload staged at ${stagedPath} could not become it: ${reasonOf(failure)}`);
			return false;
		}
	}

	private async copyReadAsRequester(requested: RequestedCopy): Promise<MessengerCopy> {
		const source = await this.settings.readerOf(requested.requesterKey, requested.mediaURL);
		if (!source) throw new TransferFailed(403, `the company holds no messenger key for ${requested.requesterKey || 'an unsigned reader'}`);
		return { digest: requested.digest, contentType: requested.contentType, source };
	}

	private async filled(digest: string, copyOf: () => Promise<MessengerCopy> | MessengerCopy, progress: CopyProgress): Promise<FilledCopy> {
		try {
			const copy = await copyOf();
			const path = this.pathOf(digest);
			const kept = await copyIntoStore(this.settings.access, path, copy.contentType, copy.source, {
				onProgress: progress,
				expectedDigest: digest
			});
			return { address: attachmentAddress(this.settings.access.projectURL, path), sizeBytes: kept.sizeBytes };
		} catch (failure) {
			this.reportUnfilled(digest, reasonOf(failure));
			throw failure;
		}
	}

	private reportUnfilled(digest: string, reason: string): void {
		this.settings.report(`messenger copy ${digest} not filled: ${reason}`);
	}
}
