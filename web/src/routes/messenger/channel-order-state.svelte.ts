import { isSupabaseConfigured } from '$lib/supabase';
import { messengerCacheScope, requireCurrentMessengerScope, type MessengerCacheScope } from '$lib/messenger/cache-scope';
import {
	channelOrderOfMine,
	keepChannelOrder,
	pinChannel,
	unpinChannel,
	type ChannelOrder
} from '$lib/messenger/channel-order-api';
import { keepCachedChannelOrder, noChannelOrder, readCachedChannelOrder } from '$lib/messenger/channel-order-cache';
import { moveChannel } from './channel-order';

export class ChannelOrderState {
	pinnedChannels = $state<string[]>([]);
	channelOrder = $state<string[]>([]);

	private readonly isKeptOnTheRecord = isSupabaseConfigured();
	private readSequence = 0;
	private writeSequence = 0;

	isPinned(channelID: string): boolean {
		return this.pinnedChannels.includes(channelID);
	}

	showCached(scope: MessengerCacheScope): void {
		this.adopt(readCachedChannelOrder(scope));
	}

	async load(): Promise<void> {
		const sequence = ++this.readSequence;
		const writesBefore = this.writeSequence;
		try {
			const scope = await messengerCacheScope();
			const answered = this.isKeptOnTheRecord ? await channelOrderOfMine() : readCachedChannelOrder(scope);
			await requireCurrentMessengerScope(scope);
			if (sequence !== this.readSequence || writesBefore !== this.writeSequence) return;
			this.adopt(answered);
			keepCachedChannelOrder(scope, answered);
		} catch (failure) {
			console.warn('the channel order did not load', failure);
		}
	}

	reset(): void {
		this.readSequence += 1;
		this.writeSequence += 1;
		this.adopt(noChannelOrder);
	}

	switchPinned(channelID: string): Promise<void> {
		if (this.isPinned(channelID)) {
			const pinnedChannels = this.pinnedChannels.filter((pinnedID) => pinnedID !== channelID);
			return this.write({ ...this.current(), pinnedChannels }, () => unpinChannel(channelID));
		}
		const pinnedChannels = [...this.pinnedChannels, channelID];
		return this.write({ ...this.current(), pinnedChannels }, () => pinChannel(channelID));
	}

	movePinned(draggedID: string, targetID: string): Promise<void> {
		const pinnedChannels = moveChannel(this.pinnedChannels, draggedID, targetID);
		if (pinnedChannels === this.pinnedChannels) return Promise.resolve();
		return this.write({ ...this.current(), pinnedChannels }, () => keepChannelOrder({ pinnedChannels }));
	}

	moveChannel(orderedIDs: string[], draggedID: string, targetID: string): Promise<void> {
		const channelOrder = moveChannel(orderedIDs, draggedID, targetID);
		if (channelOrder === orderedIDs) return Promise.resolve();
		return this.write({ ...this.current(), channelOrder }, () => keepChannelOrder({ channelOrder }));
	}

	private current(): ChannelOrder {
		return { pinnedChannels: this.pinnedChannels, channelOrder: this.channelOrder };
	}

	private adopt(order: ChannelOrder): void {
		this.pinnedChannels = order.pinnedChannels;
		this.channelOrder = order.channelOrder;
	}

	private async write(next: ChannelOrder, send: () => Promise<ChannelOrder>): Promise<void> {
		const before = this.current();
		const sequence = ++this.writeSequence;
		this.adopt(next);
		try {
			const scope = await messengerCacheScope();
			const kept = this.isKeptOnTheRecord ? await send() : next;
			await requireCurrentMessengerScope(scope);
			if (sequence !== this.writeSequence) return;
			this.adopt(kept);
			keepCachedChannelOrder(scope, kept);
		} catch (refusal) {
			if (sequence === this.writeSequence) this.adopt(before);
			throw refusal;
		}
	}
}
