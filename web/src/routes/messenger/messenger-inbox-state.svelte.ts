import { bridgeInbox, type InboxEntry } from '$lib/messenger/channel-over-bridge';
import { messengerCacheScope, requireCurrentMessengerScope } from '$lib/messenger/cache-scope';
import { INBOX_PAGE_SIZE, appendOlderInbox, keepCachedInbox, mergeInbox } from '$lib/messenger/inbox-cache';
import { markThreadRead, type MessengerInboxWindow } from '$lib/messenger/messenger-api';

export type InboxFilter = 'all' | 'unread';

export class MessengerInboxState {
	entries = $state<InboxEntry[]>([]);
	isLoading = $state(false);
	hasLoaded = $state(false);
	error = $state('');
	filter = $state<InboxFilter>('all');
	hasMoreBefore = $state(true);
	isLoadingOlder = $state(false);

	readonly unreadCount = $derived(this.entries.filter((entry) => entry.isUnread).length);
	readonly shown = $derived(this.filter === 'unread' ? this.entries.filter((entry) => entry.isUnread) : this.entries);

	private readSequence = 0;
	private isRefreshWanted = false;

	showCached(cached: InboxEntry[]): void {
		if (this.hasLoaded || cached.length === 0) return;
		this.entries = cached;
		this.hasLoaded = true;
	}

	async load(): Promise<void> {
		const sequence = ++this.readSequence;
		this.isLoading = true;
		this.error = '';
		try {
			const scope = await messengerCacheScope();
			const window = this.windowSinceNewest();
			const answer = await bridgeInbox(window);
			await requireCurrentMessengerScope(scope);
			if (sequence !== this.readSequence) return;
			if (!window || answer.hasMoreBefore) {
				this.entries = answer.entries;
				this.hasMoreBefore = answer.hasMoreBefore;
			} else {
				this.entries = mergeInbox(this.entries, answer, window.heldIDs);
			}
			this.hasLoaded = true;
			keepCachedInbox(scope, this.entries);
		} catch (failure) {
			console.warn('the inbox did not load', failure);
			if (sequence === this.readSequence) this.error = failure instanceof Error ? failure.message : String(failure);
		} finally {
			if (sequence === this.readSequence) this.isLoading = false;
		}
	}

	async loadOlder(): Promise<void> {
		const oldest = this.entries.at(-1);
		if (this.isLoadingOlder || !this.hasMoreBefore || !oldest) return;
		this.isLoadingOlder = true;
		try {
			const answer = await bridgeInbox({ until: oldest.sentAt });
			if (this.entries.at(-1)?.id !== oldest.id) return;
			this.entries = appendOlderInbox(this.entries, answer.entries);
			this.hasMoreBefore = answer.hasMoreBefore;
		} catch (failure) {
			console.warn('the older inbox did not load', failure);
		} finally {
			this.isLoadingOlder = false;
		}
	}

	async markRead(chosen: InboxEntry): Promise<void> {
		if (!chosen.isUnread) return;
		this.entries = this.entries.map((entry) => (entry.id === chosen.id ? { ...entry, isUnread: false } : entry));
		if (!chosen.threadRootId) return;
		await markThreadRead(chosen.conversationID, chosen.threadRootId, chosen.sentAt);
	}

	async refresh(): Promise<void> {
		if (this.isLoading) {
			this.isRefreshWanted = true;
			return;
		}
		do {
			this.isRefreshWanted = false;
			await this.load();
		} while (this.isRefreshWanted);
	}

	reset(): void {
		this.readSequence += 1;
		this.isRefreshWanted = false;
		this.entries = [];
		this.hasLoaded = false;
		this.isLoading = false;
		this.isLoadingOlder = false;
		this.hasMoreBefore = true;
		this.error = '';
	}

	private windowSinceNewest(): MessengerInboxWindow | undefined {
		const newest = this.entries[0];
		if (!this.hasLoaded || !newest) return undefined;
		return { since: newest.sentAt, heldIDs: this.entries.slice(0, INBOX_PAGE_SIZE).map((entry) => entry.id) };
	}
}
