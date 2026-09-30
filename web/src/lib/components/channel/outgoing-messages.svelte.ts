import type { ChannelMessage, ChannelParticipant } from './channel-api';
import type { OutgoingMessage } from './channel-composer.svelte';

export type OutgoingEntry = {
	message: ChannelMessage;
	draft: OutgoingMessage;
	channelID: string | undefined;
	threadRootID: string | undefined;
	hasFailed: boolean;
};

export type OutgoingMessages = ReturnType<typeof createOutgoingMessages>;

export function createOutgoingMessages(options: {
	sender: () => ChannelParticipant;
	deliver: (entry: OutgoingEntry) => Promise<void>;
	afterDelivered: (entry: OutgoingEntry) => Promise<void>;
}) {
	let entries = $state<OutgoingEntry[]>([]);
	let serial = 0;

	function replace(messageID: string, change: Partial<OutgoingEntry>): void {
		entries = entries.map((entry) => (entry.message.id === messageID ? { ...entry, ...change } : entry));
	}

	function forget(messageID: string): void {
		entries = entries.filter((entry) => entry.message.id !== messageID);
	}

	async function attempt(entry: OutgoingEntry): Promise<void> {
		try {
			await options.deliver(entry);
		} catch {
			replace(entry.message.id, { hasFailed: true });
			return;
		}
		await options.afterDelivered(entry);
		forget(entry.message.id);
	}

	return {
		messagesIn(channelID: string | undefined): ChannelMessage[] {
			return entries.filter((entry) => entry.channelID === channelID).map((entry) => entry.message);
		},
		hasFailed(messageID: string): boolean {
			return entries.some((entry) => entry.message.id === messageID && entry.hasFailed);
		},
		send(draft: OutgoingMessage, channelID: string | undefined, threadRootID?: string): Promise<void> {
			const entry: OutgoingEntry = {
				message: {
					id: `pending-${serial++}`,
					threadRootId: threadRootID,
					sender: options.sender(),
					text: draft.text || draft.attachmentSummary,
					sentAt: new Date().toISOString()
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
			forget(messageID);
		}
	};
}
