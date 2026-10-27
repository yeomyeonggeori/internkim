import { channelOrderStorageKey, type MessengerCacheScope } from './cache-scope';
import type { ChannelOrder } from './channel-order-api';

export const noChannelOrder: ChannelOrder = { pinnedChannels: [], channelOrder: [] };

function channelIDsOf(value: unknown): string[] {
	return Array.isArray(value) ? value.filter((channelID): channelID is string => typeof channelID === 'string') : [];
}

export function readCachedChannelOrder(scope: MessengerCacheScope): ChannelOrder {
	try {
		if (typeof localStorage === 'undefined') return noChannelOrder;
		const cached: unknown = JSON.parse(localStorage.getItem(channelOrderStorageKey(scope)) ?? 'null');
		if (typeof cached !== 'object' || cached === null) return noChannelOrder;
		return {
			pinnedChannels: channelIDsOf(Reflect.get(cached, 'pinnedChannels')),
			channelOrder: channelIDsOf(Reflect.get(cached, 'channelOrder'))
		};
	} catch {
		return noChannelOrder;
	}
}

export function keepCachedChannelOrder(scope: MessengerCacheScope, order: ChannelOrder): void {
	try {
		if (typeof localStorage !== 'undefined') localStorage.setItem(channelOrderStorageKey(scope), JSON.stringify(order));
	} catch (refusal) {
		console.warn('the messenger could not keep its channel order on this device', refusal);
	}
}
