import type { ChannelMessage } from './channel-api';

export type ChannelMessageGroup = {
	id: string;
	senderID: string;
	items: ChannelMessage[];
};

export function groupConsecutiveMessages(messages: ChannelMessage[]): ChannelMessageGroup[] {
	const groups: ChannelMessageGroup[] = [];
	for (const message of messages) {
		const lastGroup = groups.at(-1);
		const lastMessageOpensAThread = lastGroup?.items.at(-1)?.thread !== undefined;
		const isSameSender = lastGroup?.senderID === message.sender.id;
		if (lastGroup && isSameSender && !lastMessageOpensAThread) lastGroup.items.push(message);
		else groups.push({ id: message.id, senderID: message.sender.id, items: [message] });
	}
	return groups;
}
