export type ReadThrough = (conversationID: string, messageID: string, threadRootID?: string) => Promise<void>;

export type ReadMarker = (conversationID: string, messageID: string, threadRootID?: string) => Promise<boolean>;

export function createReadMarker(mark: ReadThrough): ReadMarker {
	const markedThrough = new Map<string, string>();
	return async (conversationID, messageID, threadRootID) => {
		const context = `${conversationID} ${threadRootID ?? ''}`;
		const previous = markedThrough.get(context);
		if (previous === messageID) return false;
		markedThrough.set(context, messageID);
		try {
			await mark(conversationID, messageID, threadRootID);
		} catch (failure) {
			if (previous === undefined) markedThrough.delete(context);
			else markedThrough.set(context, previous);
			throw failure;
		}
		return true;
	};
}

export type ReadableMessage = { id: string; sentAt: string; threadRootId?: string };

const pendingMessagePrefix = 'pending-';

export function latestSettledOf(messages: ReadableMessage[]): ReadableMessage | undefined {
	return latestOf(messages.filter((message) => !message.id.startsWith(pendingMessagePrefix)));
}

export function latestSentAt(messages: ReadableMessage[]): string {
	return latestOf(messages)?.sentAt ?? '';
}

function latestOf(messages: ReadableMessage[]): ReadableMessage | undefined {
	let latest: ReadableMessage | undefined;
	let latestMilliseconds = Number.NEGATIVE_INFINITY;
	for (const message of messages) {
		const sentAtMilliseconds = Date.parse(message.sentAt);
		if (sentAtMilliseconds > latestMilliseconds) {
			latest = message;
			latestMilliseconds = sentAtMilliseconds;
		}
	}
	return latest;
}
