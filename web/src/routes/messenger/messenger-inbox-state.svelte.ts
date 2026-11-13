import { bridgeInbox, type InboxEntry } from '$lib/messenger/channel-over-bridge';
import { messengerCacheScope, requireCurrentMessengerScope } from '$lib/messenger/cache-scope';
import { appendOlderInbox, keepCachedInbox } from '$lib/messenger/inbox-cache';
import { markReadThrough, type MessengerReadCount } from '$lib/messenger/messenger-api';

export type InboxFilter = 'all' | 'unread';

export class MessengerInboxState {
	entries = $state<InboxEntry[]>([]);
	isLoading = $state(false);
	hasLoaded = $state(false);
	error = $state('');
	filter = $state<InboxFilter>('all');
	hasMoreBefore = $state(true);
	isLoadingOlder = $state(false);

	readonly unread: MessengerReadCount = $derived({ status: 'exact', value: this.entries.filter((entry) => entry.isUnread).length });
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
			const answer = await bridgeInbox();
			await requireCurrentMessengerScope(scope);
			if (sequence !== this.readSequence) return;
			this.entries = answer.entries;
			this.hasMoreBefore = answer.hasMoreBefore;
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
		const oldestMention = this.entries.findLast((entry) => entry.reason === 'mention');
		if (this.isLoadingOlder || !this.hasMoreBefore || !oldestMention) return;
		this.isLoadingOlder = true;
		try {
			const answer = await bridgeInbox(oldestMention.sentAt);
			if (this.entries.findLast((entry) => entry.reason === 'mention')?.id !== oldestMention.id) return;
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
		await markReadThrough(chosen.conversationID, chosen.id, chosen.threadRootId);
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

}
