import type { ChannelMessage } from './channel-api';
import { dateKeyOf, dateLabel } from './channel-time';

export type ChannelMessageGroup = {
	id: string;
	senderID: string;
	items: ChannelMessage[];
};

export type ChannelTimelineItem =
	| { kind: 'date'; id: string; label: string }
	| ({ kind: 'group' } & ChannelMessageGroup);

export function groupConsecutiveMessages(messages: ChannelMessage[]): ChannelMessageGroup[] {
	const groups: ChannelMessageGroup[] = [];
	for (const message of messages) {
		const lastGroup = groups.at(-1);
		const lastMessage = lastGroup?.items.at(-1);
		const lastMessageOpensAThread = lastMessage?.thread !== undefined;
		const isSameSender = lastGroup?.senderID === message.sender.id;
		const isSameDay = lastMessage !== undefined && dateKeyOf(lastMessage.sentAt) === dateKeyOf(message.sentAt);
		if (lastGroup && isSameSender && isSameDay && !lastMessageOpensAThread) lastGroup.items.push(message);
		else groups.push(standaloneGroup(message));
	}
	return groups;
}

export function datedTimeline(groups: ChannelMessageGroup[]): ChannelTimelineItem[] {
	const items: ChannelTimelineItem[] = [];
	let previousDateKey = '';
	for (const group of groups) {
		const firstSentAt = group.items[0].sentAt;
		const dateKey = dateKeyOf(firstSentAt);
		if (dateKey !== previousDateKey) {
			items.push({ kind: 'date', id: `date-${dateKey}`, label: dateLabel(firstSentAt) });
			previousDateKey = dateKey;
		}
		items.push({ kind: 'group', ...group });
	}
	return items;
}

export function standaloneGroup(message: ChannelMessage): ChannelMessageGroup {
	return { id: message.id, senderID: message.sender.id, items: [message] };
}
