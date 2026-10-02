import type { ReadableMessage } from '$lib/messenger/conversation-read-marker';

export type JumpToLatestText = {
	jumpToLatest: string;
	unseenMessageOne: string;
	unseenMessageMany: string;
};

export function unseenMessageCount<Message extends ReadableMessage>(
	messages: Message[],
	seenThroughSentAt: string,
	isMine: (message: Message) => boolean
): number {
	const seenThroughMilliseconds = Date.parse(seenThroughSentAt);
	if (Number.isNaN(seenThroughMilliseconds)) return 0;
	return messages.filter(
		(message) =>
			!message.threadRootId && !isMine(message) && Date.parse(message.sentAt) > seenThroughMilliseconds
	).length;
}

export function jumpToLatestLabel(unseenCount: number, text: JumpToLatestText): string {
	if (unseenCount === 0) return text.jumpToLatest;
	if (unseenCount === 1) return text.unseenMessageOne;
	return text.unseenMessageMany.replace('{count}', String(unseenCount));
}
