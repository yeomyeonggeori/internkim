export type MarkConversationRead = (conversationID: string, readAt: string) => Promise<void>;

export type ConversationReadMarker = (conversationID: string, readAt: string) => Promise<boolean>;

export function createConversationReadMarker(mark: MarkConversationRead): ConversationReadMarker {
	const markedThrough = new Map<string, number>();
	return async (conversationID, readAt) => {
		const readAtMilliseconds = Date.parse(readAt);
		if (Number.isNaN(readAtMilliseconds)) return false;
		const previous = markedThrough.get(conversationID);
		if (previous !== undefined && previous >= readAtMilliseconds) return false;
		markedThrough.set(conversationID, readAtMilliseconds);
		try {
			await mark(conversationID, readAt);
		} catch (failure) {
			if (previous === undefined) markedThrough.delete(conversationID);
			else markedThrough.set(conversationID, previous);
			throw failure;
		}
		return true;
	};
}

export type ReadableMessage = { id: string; sentAt: string; threadRootId?: string };

const pendingMessagePrefix = 'pending-';

export function latestSentAtOf(messages: ReadableMessage[]): string {
	let latest = '';
	let latestMilliseconds = Number.NEGATIVE_INFINITY;
	for (const message of messages) {
		if (message.threadRootId || message.id.startsWith(pendingMessagePrefix)) continue;
		const sentAtMilliseconds = Date.parse(message.sentAt);
		if (sentAtMilliseconds > latestMilliseconds) {
			latest = message.sentAt;
			latestMilliseconds = sentAtMilliseconds;
		}
	}
	return latest;
}
