import type { ChannelOutgoingAttachment } from './channel-api';
import { fileToAttachment } from './channel-attachments';

export type PendingAttachment = {
	id: string;
	previewURL: string;
	isImage: boolean;
	sizeBytes: number;
	attachment: ChannelOutgoingAttachment;
};

export class PendingAttachments {
	items = $state<PendingAttachment[]>([]);
	#serial = 0;

	get outgoing(): ChannelOutgoingAttachment[] {
		return this.items.map((pending) => pending.attachment);
	}

	async add(files: File[]): Promise<void> {
		const built: PendingAttachment[] = [];
		for (const file of files) {
			built.push({
				id: `attachment-${this.#serial++}`,
				previewURL: URL.createObjectURL(file),
				isImage: file.type.startsWith('image/'),
				sizeBytes: file.size,
				attachment: await fileToAttachment(file)
			});
		}
		this.items = [...this.items, ...built];
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
}
