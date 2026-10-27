import { invokeTool } from '$lib/public-api-call';

export type ChannelOrder = { pinnedChannels: string[]; channelOrder: string[] };

function channelIDsIn(answer: object, field: keyof ChannelOrder): string[] {
	const listed: unknown = Reflect.get(answer, field);
	if (!Array.isArray(listed) || !listed.every((channelID): channelID is string => typeof channelID === 'string')) {
		throw new Error(`the channel order answer holds no ${field} list`);
	}
	return listed;
}

function channelOrderAnswered(answer: unknown): ChannelOrder {
	if (typeof answer !== 'object' || answer === null) throw new Error('the channel order answer is not an object');
	return { pinnedChannels: channelIDsIn(answer, 'pinnedChannels'), channelOrder: channelIDsIn(answer, 'channelOrder') };
}

export async function channelOrderOfMine(): Promise<ChannelOrder> {
	return channelOrderAnswered(await invokeTool<unknown>('channel_order_get', {}));
}

export async function pinChannel(channelID: string): Promise<ChannelOrder> {
	return channelOrderAnswered(await invokeTool<unknown>('channel_pin', { channelID }));
}

export async function unpinChannel(channelID: string): Promise<ChannelOrder> {
	return channelOrderAnswered(await invokeTool<unknown>('channel_unpin', { channelID }));
}

export async function keepChannelOrder(change: Partial<ChannelOrder>): Promise<ChannelOrder> {
	return channelOrderAnswered(await invokeTool<unknown>('channel_order_set', change));
}
