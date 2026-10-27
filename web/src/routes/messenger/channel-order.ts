import type { ChannelSummary } from '$lib/components/channel/channel-api';

const pinnedToTop = ['광장', '잡담'];
const pinnedToBottom = ['일정', '업무', '근태'];

function defaultRank(name: string): number {
	const topIndex = pinnedToTop.indexOf(name);
	if (topIndex !== -1) return topIndex;
	const bottomIndex = pinnedToBottom.indexOf(name);
	if (bottomIndex !== -1) return 1000 + bottomIndex;
	return 500;
}

export function orderChannels(channels: ChannelSummary[], userOrder: string[]): ChannelSummary[] {
	const userRank = new Map(userOrder.map((channelID, index) => [channelID, index]));
	return [...channels].sort((first, second) => {
		const firstRank = userRank.get(first.id);
		const secondRank = userRank.get(second.id);
		if (firstRank !== undefined && secondRank !== undefined) return firstRank - secondRank;
		if (firstRank !== undefined) return -1;
		if (secondRank !== undefined) return 1;
		return defaultRank(first.name) - defaultRank(second.name);
	});
}

export function pinnedChannelsOf(channels: ChannelSummary[], pinnedChannelIDs: string[]): ChannelSummary[] {
	const channelOf = new Map(channels.map((channel) => [channel.id, channel]));
	return pinnedChannelIDs.flatMap((channelID) => channelOf.get(channelID) ?? []);
}

export function orderDirectMessages(directMessages: ChannelSummary[]): ChannelSummary[] {
	return [
		...directMessages.filter((conversation) => conversation.isWithTheAgent === true),
		...directMessages.filter((conversation) => conversation.isWithTheAgent !== true)
	];
}

export function moveChannel(orderedIDs: string[], draggedID: string, targetID: string): string[] {
	const fromIndex = orderedIDs.indexOf(draggedID);
	const toIndex = orderedIDs.indexOf(targetID);
	if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) return orderedIDs;
	const next = [...orderedIDs];
	next.splice(fromIndex, 1);
	next.splice(toIndex, 0, draggedID);
	return next;
}
