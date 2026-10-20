<script lang="ts">
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import UnreadDot from '$lib/components/unread-dot.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Item from '$lib/components/ui/item/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import type { Snippet } from 'svelte';
	import type { ChannelSummary } from '$lib/components/channel/channel-api';
	import { relativeTime } from '$lib/components/channel/channel-time';
	import { messengerInboxText } from '$lib/i18n/messenger-inbox-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { InboxEntry } from '$lib/messenger/channel-over-bridge';
	import { messagePlainText } from '$lib/messenger/message-plain-text';
	import { cn } from '$lib/utils.js';
	import MessengerInboxSkeleton from './messenger-inbox-skeleton.svelte';
	import type { MessengerInboxState } from './messenger-inbox-state.svelte';

	let {
		inbox,
		conversations,
		chosenID,
		choose,
		leading,
		class: className = ''
	}: {
		inbox: MessengerInboxState;
		conversations: ChannelSummary[];
		chosenID: string | undefined;
		choose: (entry: InboxEntry) => void;
		leading?: Snippet;
		class?: string;
	} = $props();

	const text = createPageText(messengerInboxText);

	const conversationOf = $derived(new Map(conversations.map((conversation) => [conversation.id, conversation])));

	function reasonLabel(entry: InboxEntry): string {
		if (entry.reason === 'direct') return text.direct;
		if (entry.reason === 'mention') return text.mention;
		return text.threadReply;
	}

	function placeLabel(entry: InboxEntry): string {
		const conversation = conversationOf.get(entry.conversationID);
		if (!conversation || conversation.kind === 'dm') return '';
		return `# ${conversation.name}`;
	}
</script>

<aside class={cn('flex min-h-0 flex-col', className)} aria-label={text.inbox}>
	<header class="flex h-12 shrink-0 items-center gap-2 border-b px-2 sm:h-14 sm:px-4">
		{@render leading?.()}
		<Tabs.Root value={inbox.filter} onValueChange={(value) => (inbox.filter = value === 'unread' ? 'unread' : 'all')}>
			<Tabs.List aria-label={text.inbox}>
				<Tabs.Trigger value="all">{text.all}</Tabs.Trigger>
				<Tabs.Trigger value="unread">{text.unread}</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>
	</header>
	{#if inbox.error && !inbox.hasLoaded}
		<div class="space-y-2 px-4 py-3">
			<p role="alert" class="text-destructive text-sm">{text.loadFailed}</p>
			<Button variant="outline" size="sm" onclick={() => inbox.load()}>{text.retry}</Button>
		</div>
	{:else if !inbox.hasLoaded}
		<MessengerInboxSkeleton label={text.loading} />
	{:else if inbox.shown.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon"><InboxIcon /></Empty.Media>
				<Empty.Title>{inbox.filter === 'unread' ? text.emptyUnread : text.empty}</Empty.Title>
			</Empty.Header>
		</Empty.Root>
	{:else}
		<Item.Group class="relative min-h-0 flex-1 gap-1 overflow-y-auto p-2">
			{#each inbox.shown as entry (entry.id)}
				{@const place = placeLabel(entry)}
				<Item.Root size="sm" class={cn('hover:bg-muted text-left', entry.id === chosenID && 'bg-muted')}>
					{#snippet child({ props })}
						<button {...props} type="button" data-inbox-entry={entry.id} onclick={() => choose(entry)}>
							<Item.Media>
								<PersonAvatar
									name={entry.sender.name}
									seed={entry.sender.id || entry.sender.name}
									image={entry.sender.avatarURL ?? ''}
									memberID={entry.sender.memberID ?? ''}
									externalID={entry.sender.externalID ?? ''}
									class="size-8"
								/>
							</Item.Media>
							<Item.Content class="min-w-0">
								<Item.Title class="w-full">
									<span class={cn('truncate', entry.isUnread && 'font-semibold')}>{entry.sender.name || text.unknownAuthor}</span>
									{#if entry.isUnread}
										<UnreadDot label={text.unread} />
									{/if}
									<time class="text-muted-foreground ml-auto shrink-0 text-xs font-normal">{relativeTime(entry.sentAt)}</time>
								</Item.Title>
								<p class="text-muted-foreground truncate text-xs">
									{place ? `${reasonLabel(entry)} · ${place}` : reasonLabel(entry)}
								</p>
								<Item.Description class={cn('line-clamp-2', entry.isUnread && 'text-foreground')}>{messagePlainText(entry.text)}</Item.Description>
							</Item.Content>
						</button>
					{/snippet}
				</Item.Root>
			{/each}
		</Item.Group>
	{/if}
</aside>
