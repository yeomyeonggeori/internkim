import type { ChannelSummary } from '$lib/components/channel/channel-api';
import type { CompanyEvent } from '$lib/company-event';
import { onCompanyEvent } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import {
	adoptRememberedMessengerScope,
	messengerCacheScope,
	onMessengerCacheReset,
	requireCurrentMessengerScope
} from './cache-scope';
import type { InboxEntry } from './channel-over-bridge';
import { keepCachedConversations, readCachedConversations } from './conversation-list-cache';
import { keepCachedInbox, readCachedInbox } from './inbox-cache';
import { messengerUnreadTotal } from './unread-total';

type OpenMessenger = {
	conversations: () => ChannelSummary[] | undefined;
	inboxEntries: () => InboxEntry[] | undefined;
	mutedIDs: () => ReadonlySet<string>;
};

class MessengerUnread {
	private conversations = $state.raw<ChannelSummary[]>([]);
	private inboxEntries = $state.raw<InboxEntry[]>([]);
	private mutedIDs = $state.raw<ReadonlySet<string>>(new Set());
	readonly total = $derived(messengerUnreadTotal(this.conversations, this.inboxEntries, this.mutedIDs));
	arrivals = $state(0);

	private stopFollowing: (() => void) | null = null;
	private openMessengers = 0;
	private reading: Promise<void> | null = null;
	private isRereadWanted = false;
	private hasReadMuted = false;
	private hasCounted = false;
	private readonly conversationsNotMine = new Set<string>();

	follow = (): void => {
		if (this.stopFollowing) return;
		const stopResets = onMessengerCacheReset(this.startOver);
		const stopArrivals = isSupabaseConfigured() ? onCompanyEvent(this.rereadOnArrival) : () => {};
		this.stopFollowing = () => {
			stopResets();
			stopArrivals();
		};
		void this.showRemembered().then(this.reread);
	};

	forget = (): void => {
		this.stopFollowing?.();
		this.stopFollowing = null;
		this.clear();
	};

	showConversations = (conversations: ChannelSummary[]): void => {
		this.conversations = conversations;
		this.hasCounted = true;
	};

	showInbox = (inboxEntries: InboxEntry[]): void => {
		this.inboxEntries = inboxEntries;
	};

	showMuted = (mutedIDs: ReadonlySet<string>): void => {
		this.mutedIDs = mutedIDs;
		this.hasReadMuted = true;
	};

	leaveReadingToMessenger = (): (() => void) => {
		this.openMessengers += 1;
		return () => {
			this.openMessengers -= 1;
		};
	};

	private startOver = (): void => {
		this.clear();
		if (this.openMessengers === 0) void this.reread();
	};

	private clear(): void {
		this.conversations = [];
		this.inboxEntries = [];
		this.mutedIDs = new Set();
		this.hasReadMuted = false;
		this.hasCounted = false;
		this.conversationsNotMine.clear();
	}

	private rereadOnArrival = (event: CompanyEvent): void => {
		if (event.kind !== 'message.arrived' || !event.conversationID || this.openMessengers > 0) return;
		const conversationID = event.conversationID;
		if (this.conversationsNotMine.has(conversationID)) return;
		void this.reread().then(() => {
			if (!this.conversations.some((conversation) => conversation.id === conversationID)) {
				this.conversationsNotMine.add(conversationID);
			}
		});
	};

	private reread = (): Promise<void> => {
		if (this.reading) {
			this.isRereadWanted = true;
			return this.reading;
		}
		this.reading = this.readUntilSettled().finally(() => {
			this.reading = null;
		});
		return this.reading;
	};

	private async readUntilSettled(): Promise<void> {
		do {
			this.isRereadWanted = false;
			await this.read();
		} while (this.isRereadWanted);
	}

	private async showRemembered(): Promise<void> {
		try {
			const remembered = await adoptRememberedMessengerScope();
			if (!remembered || this.openMessengers > 0) return;
			this.conversations = readCachedConversations(remembered);
			if (isSupabaseConfigured()) this.inboxEntries = readCachedInbox(remembered);
		} catch (failure) {
			console.warn('the rail did not show the unread messages this device remembers', failure);
		}
	}

	private async readMutedOnce(): Promise<ReadonlySet<string> | undefined> {
		if (!isSupabaseConfigured() || this.hasReadMuted) return undefined;
		try {
			return await readMutedIDs();
		} catch (failure) {
			console.warn('the rail counts muted conversations because it did not read which are muted', failure);
			return undefined;
		}
	}

	private async read(): Promise<void> {
		try {
			const scope = await messengerCacheScope();
			const [conversations, inboxEntries, mutedIDs] = await Promise.all([
				readConversations(),
				isSupabaseConfigured() ? readInboxEntries() : this.inboxEntries,
				this.readMutedOnce()
			]);
			await requireCurrentMessengerScope(scope);
			if (this.openMessengers > 0) return;
			if (mutedIDs) this.showMuted(mutedIDs);
			const before = this.total.addressedToMe;
			this.conversations = conversations;
			this.inboxEntries = inboxEntries;
			if (this.hasCounted && this.total.addressedToMe > before) this.arrivals += 1;
			this.hasCounted = true;
			keepCachedConversations(scope, conversations);
			if (isSupabaseConfigured()) keepCachedInbox(scope, inboxEntries);
		} catch (failure) {
			console.warn('the rail did not count the unread messages', failure);
		}
	}
}

export const messengerUnread = new MessengerUnread();

async function readConversations(): Promise<ChannelSummary[]> {
	const { fetchConversations } = await import('$lib/components/channel/channel-api');
	return fetchConversations();
}

async function readInboxEntries(): Promise<InboxEntry[]> {
	const { bridgeInbox } = await import('./channel-over-bridge');
	return (await bridgeInbox()).entries;
}

async function readMutedIDs(): Promise<ReadonlySet<string>> {
	const { mutedConversations } = await import('$lib/notifications/muted-conversations');
	return mutedConversations();
}

export function countUnreadFromOpenMessenger(open: OpenMessenger): void {
	let isInitialMuted = true;

	$effect(() => messengerUnread.leaveReadingToMessenger());

	$effect(() => {
		const conversations = open.conversations();
		if (conversations) messengerUnread.showConversations(conversations);
	});

	$effect(() => {
		const inboxEntries = open.inboxEntries();
		if (inboxEntries) messengerUnread.showInbox(inboxEntries);
	});

	$effect(() => {
		const mutedIDs = open.mutedIDs();
		if (isInitialMuted) {
			isInitialMuted = false;
			return;
		}
		messengerUnread.showMuted(mutedIDs);
	});
}
