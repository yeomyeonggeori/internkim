import type { ChannelOutgoingAttachment } from './channel-api';
import { fileToAttachment } from './channel-attachments';

export type PendingAttachmentState = 'processing' | 'error' | 'done';

export type PendingAttachment = {
	id: string;
	previewURL: string;
	isImage: boolean;
	filename: string;
	contentType: string;
	sizeBytes: number;
	state: PendingAttachmentState;
	failure: string;
	attachment: ChannelOutgoingAttachment | null;
};

export class PendingAttachments {
	items = $state<PendingAttachment[]>([]);
	#serial = 0;

	get outgoing(): ChannelOutgoingAttachment[] {
		return this.items.flatMap((pending) => (pending.attachment ? [pending.attachment] : []));
	}

	get isReady(): boolean {
		return this.items.every((pending) => pending.state === 'done');
	}

	async add(files: File[]): Promise<void> {
		const added = files.map((file) => ({ file, pending: this.#pendingFor(file) }));
		this.items = [...this.items, ...added.map(({ pending }) => pending)];
		await Promise.all(added.map(({ file, pending }) => this.#prepare(file, pending.id)));
	}

	remove(id: string): void {
		const removed = this.items.find((pending) => pending.id === id);
		if (removed) URL.revokeObjectURL(removed.previewURL);
		this.items = this.items.filter((pending) => pending.id !== id);
	}

	clear(): void {
		for (const pending of this.items) URL.revokeObjectURL(pending.previewURL);
		this.items = [];
	}

	#pendingFor(file: File): PendingAttachment {
		return {
			id: `attachment-${this.#serial++}`,
			previewURL: URL.createObjectURL(file),
			isImage: file.type.startsWith('image/'),
			filename: file.name,
			contentType: file.type,
			sizeBytes: file.size,
			state: 'processing',
			failure: '',
			attachment: null
		};
	}

	async #prepare(file: File, id: string): Promise<void> {
		try {
			const attachment = await fileToAttachment(file);
			this.#update(id, {
				state: 'done',
				filename: attachment.filename,
				contentType: attachment.contentType,
				sizeBytes: attachment.content.size,
				attachment
			});
		} catch (error) {
			if (!(error instanceof Error)) throw error;
			this.#update(id, { state: 'error', failure: error.message });
		}
	}

	#update(id: string, changes: Partial<PendingAttachment>): void {
		this.items = this.items.map((pending) => (pending.id === id ? { ...pending, ...changes } : pending));
	}
}
