import { feelHaptic } from '$lib/native-shell/haptics';
import { attachmentKindOf } from '$lib/messenger/attachment-kind';
import type { ChannelMessage, ChannelMessageAttachment, ChannelParticipant } from './channel-api';
import type { OutgoingMessage } from './channel-composer.svelte';

export type OutgoingEntry = {
	message: ChannelMessage;
	draft: OutgoingMessage;
	channelID: string | undefined;
	threadRootID: string | undefined;
	hasFailed: boolean;
	deliveredSerial?: number;
};

export type OutgoingMessages = ReturnType<typeof createOutgoingMessages>;

function previewsOf(messageID: string, draft: OutgoingMessage): ChannelMessageAttachment[] {
	return draft.attachments.map((attachment, index) => ({
		kind: attachmentKindOf(attachment.contentType),
		url: `${messageID}-attachment-${index}`,
		source: URL.createObjectURL(attachment.content),
		filename: attachment.filename,
		mimeType: attachment.contentType,
		sizeBytes: attachment.content.size
	}));
}

function releasePreviews(entry: OutgoingEntry): void {
	for (const attachment of entry.message.attachments ?? []) URL.revokeObjectURL(attachment.source ?? '');
}

function shownMessageOf(entry: OutgoingEntry): ChannelMessage {
	const isUploading = !entry.hasFailed && entry.deliveredSerial === undefined;
	return {
		...entry.message,
		attachments: entry.message.attachments?.map((attachment) => ({ ...attachment, isUploading }))
	};
}

export function createOutgoingMessages(options: {
	sender: () => ChannelParticipant;
	deliver: (entry: OutgoingEntry) => Promise<void>;
	afterDelivered: (entry: OutgoingEntry) => Promise<void>;
}) {
	let entries = $state<OutgoingEntry[]>([]);
	let serial = 0;
	let deliveredSerial = 0;

	function replace(messageID: string, change: Partial<OutgoingEntry>): void {
		entries = entries.map((entry) => (entry.message.id === messageID ? { ...entry, ...change } : entry));
	}

	function keepOnly(isKept: (entry: OutgoingEntry) => boolean): void {
		for (const entry of entries) if (!isKept(entry)) releasePreviews(entry);
		entries = entries.filter(isKept);
	}

	async function attempt(entry: OutgoingEntry): Promise<void> {
		try {
			await options.deliver(entry);
		} catch {
			replace(entry.message.id, { hasFailed: true });
			feelHaptic('failure');
			return;
		}
		deliveredSerial++;
		replace(entry.message.id, { deliveredSerial });
		await options.afterDelivered(entry);
	}

	return {
		messagesIn(channelID: string | undefined): ChannelMessage[] {
			return entries.filter((entry) => entry.channelID === channelID).map(shownMessageOf);
		},
		deliveredSoFar(): number {
			return deliveredSerial;
		},
		forgetDeliveredThrough(serialSeen: number): void {
			keepOnly((entry) => entry.deliveredSerial === undefined || entry.deliveredSerial > serialSeen);
		},
		hasFailed(messageID: string): boolean {
			return entries.some((entry) => entry.message.id === messageID && entry.hasFailed);
		},
		send(draft: OutgoingMessage, channelID: string | undefined, threadRootID?: string): Promise<void> {
			const messageID = `pending-${serial++}`;
			const entry: OutgoingEntry = {
				message: {
					id: messageID,
					threadRootId: threadRootID,
					sender: options.sender(),
					text: draft.text,
					sentAt: new Date().toISOString(),
					attachments: previewsOf(messageID, draft)
				},
				draft,
				channelID,
				threadRootID,
				hasFailed: false
			};
			entries = [...entries, entry];
			return attempt(entry);
		},
		retry(messageID: string): Promise<void> {
			const entry = entries.find((candidate) => candidate.message.id === messageID && candidate.hasFailed);
			if (!entry) return Promise.resolve();
			replace(messageID, { hasFailed: false });
			return attempt({ ...entry, hasFailed: false });
		},
		discard(messageID: string): void {
			keepOnly((entry) => entry.message.id !== messageID);
		}
	};
}
