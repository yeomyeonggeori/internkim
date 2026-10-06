import type { ChannelMessage, ChannelParticipant, ThreadSummary } from './channel-api';

export function threadRepliesByRoot(messages: ChannelMessage[]): Map<string, ChannelMessage[]> {
	const repliesByRoot = new Map<string, ChannelMessage[]>();
	for (const message of messages) {
		if (!message.threadRootId) continue;
		const replies = repliesByRoot.get(message.threadRootId) ?? [];
		replies.push(message);
		repliesByRoot.set(message.threadRootId, replies);
	}
	for (const replies of repliesByRoot.values()) {
		replies.sort((first, second) => first.sentAt.localeCompare(second.sentAt));
	}
	return repliesByRoot;
}

export function timelineMessages(
	messages: ChannelMessage[],
	repliesByRoot: Map<string, ChannelMessage[]>
): ChannelMessage[] {
	const presentIds = new Set(messages.map((message) => message.id));
	return messages
		.filter((message) => !message.threadRootId || !presentIds.has(message.threadRootId))
		.map((message) => {
			const replies = repliesByRoot.get(message.id);
			if (!replies || replies.length === 0) return message;
			return { ...message, thread: threadSummaryFor(message, replies) };
		});
}

function threadSummaryFor(rootMessage: ChannelMessage, replies: ChannelMessage[]): ThreadSummary {
	const participantsById = new Map<string, ChannelParticipant>();
	for (const message of replies) {
		participantsById.set(message.sender.id, message.sender);
	}
	return {
		replyCount: replies.length,
		lastReplyAt: replies.at(-1)?.sentAt ?? rootMessage.sentAt,
		participants: [...participantsById.values()]
	};
}
