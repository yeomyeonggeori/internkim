import { bridgeInbox, type InboxEntry } from '$lib/messenger/channel-over-bridge';
import { markThreadRead } from '$lib/messenger/messenger-api';

export type InboxFilter = 'all' | 'unread';

export class MessengerInboxState {
	entries = $state<InboxEntry[]>([]);
	isLoading = $state(false);
	hasLoaded = $state(false);
	error = $state('');
	filter = $state<InboxFilter>('all');

	readonly unreadCount = $derived(this.entries.filter((entry) => entry.isUnread).length);
	readonly shown = $derived(this.filter === 'unread' ? this.entries.filter((entry) => entry.isUnread) : this.entries);

	private readSequence = 0;
	private isRefreshWanted = false;

	async load(): Promise<void> {
		const sequence = ++this.readSequence;
		this.isLoading = true;
		this.error = '';
		try {
			const next = await bridgeInbox();
			if (sequence !== this.readSequence) return;
			this.entries = next;
			this.hasLoaded = true;
		} catch (failure) {
			console.warn('the inbox did not load', failure);
			if (sequence === this.readSequence) this.error = failure instanceof Error ? failure.message : String(failure);
		} finally {
			if (sequence === this.readSequence) this.isLoading = false;
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
		this.error = '';
	}
}
