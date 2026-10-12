<script lang="ts">
	import SearchIcon from '@lucide/svelte/icons/search';
	import SearchXIcon from '@lucide/svelte/icons/search-x';
	import XIcon from '@lucide/svelte/icons/x';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import * as Item from '$lib/components/ui/item/index.js';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { bridgeSearchConversation, type ConversationSearchMatch } from '$lib/messenger/channel-over-bridge';
	import { searchExcerpt, searchMatchTime } from '$lib/messenger/conversation-search';
	import { cn } from '$lib/utils.js';
	import { toast } from 'svelte-sonner';

	let {
		channelID,
		choose,
		close
	}: {
		channelID: string;
		choose: (match: ConversationSearchMatch) => Promise<boolean>;
		close: () => void;
	} = $props();

	const text = createPageText(channelText);
	const typingPauseMilliseconds = 300;

	let query = $state('');
	let searchedQuery = $state('');
	let matches = $state<ConversationSearchMatch[]>([]);
	let isSearching = $state(false);
	let failed = $state(false);
	let chosenID = $state('');
	let latestSearch = 0;

	$effect(() => {
		const wanted = query.trim();
		if (wanted === '') {
			latestSearch++;
			matches = [];
			searchedQuery = '';
			isSearching = false;
			failed = false;
			return;
		}
		const timer = setTimeout(() => search(wanted), typingPauseMilliseconds);
		return () => clearTimeout(timer);
	});

	async function search(wanted: string) {
		const searchNumber = ++latestSearch;
		isSearching = true;
		failed = false;
		try {
			const found = await bridgeSearchConversation(channelID, wanted);
			if (searchNumber !== latestSearch) return;
			matches = found;
			searchedQuery = wanted;
		} catch (failure: unknown) {
			if (searchNumber !== latestSearch) return;
			console.warn('conversation search failed', { channelID, failure });
			failed = true;
		} finally {
			if (searchNumber === latestSearch) isSearching = false;
		}
	}

	async function chooseMatch(match: ConversationSearchMatch) {
		chosenID = match.id;
		if (!(await choose(match))) toast.error(text.conversationSearchNotReached);
	}
</script>

<aside
	class="bg-background flex min-h-0 w-full flex-col border-l max-sm:fixed max-sm:inset-0 max-sm:z-50 sm:max-w-sm"
	aria-label={text.conversationSearch}
>
	<header class="flex h-12 shrink-0 items-center justify-between gap-2 border-b px-4 sm:h-14">
		<span class="font-semibold">{text.conversationSearch}</span>
		<Button variant="ghost" size="icon-sm" aria-label={text.closeConversationSearch} onclick={close}>
			<XIcon />
		</Button>
	</header>
	<div class="p-3">
		<InputGroup.Root>
			<InputGroup.Addon><SearchIcon /></InputGroup.Addon>
			<InputGroup.Input
				bind:value={query}
				placeholder={text.conversationSearchPlaceholder}
				aria-label={text.conversationSearchPlaceholder}
				autofocus
				onkeydown={(event) => {
					if (event.key === 'Escape') close();
				}}
			/>
			{#if isSearching}
				<InputGroup.Addon align="inline-end"><Spinner /></InputGroup.Addon>
			{:else if query !== ''}
				<InputGroup.Addon align="inline-end">
					<InputGroup.Button size="icon-xs" aria-label={text.clearConversationSearch} onclick={() => (query = '')}>
						<XIcon />
					</InputGroup.Button>
				</InputGroup.Addon>
			{/if}
		</InputGroup.Root>
		{#if failed}
			<p role="alert" class="text-destructive mt-2 px-1 text-xs">{text.conversationSearchFailed}</p>
		{:else if searchedQuery !== '' && matches.length > 0}
			<p class="text-muted-foreground mt-2 px-1 text-xs" aria-live="polite">
				{text.conversationSearchResultCount.replace('{count}', String(matches.length))}
			</p>
		{/if}
	</div>
	{#if !failed && searchedQuery !== '' && matches.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon"><SearchXIcon /></Empty.Media>
				<Empty.Title>{text.conversationSearchEmptyTitle}</Empty.Title>
				<Empty.Description>{text.conversationSearchEmptyDescription.replace('{query}', searchedQuery)}</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{:else if !failed}
		<Item.Group class="min-h-0 flex-1 gap-1 overflow-y-auto px-2 pb-2">
			{#each matches as match (match.id)}
				{@const excerpt = searchExcerpt(match.text, searchedQuery)}
				<Item.Root size="sm" class={cn('hover:bg-muted text-left', match.id === chosenID && 'bg-muted')}>
					{#snippet child({ props })}
						<button {...props} type="button" onclick={() => chooseMatch(match)}>
							<Item.Media>
								<PersonAvatar
									name={match.sender.name}
									seed={match.sender.id || match.sender.name}
									image={match.sender.avatarURL ?? ''}
									memberID={match.sender.memberID ?? ''}
									externalID={match.sender.externalID ?? ''}
									class="size-8"
								/>
							</Item.Media>
							<Item.Content class="min-w-0">
								<Item.Title>
									{match.isMine ? text.conversationSearchMe : match.sender.name}
									<time class="text-muted-foreground text-xs font-normal">{searchMatchTime(match.sentAt)}</time>
								</Item.Title>
								<Item.Description class="line-clamp-2">
									{excerpt.before}<mark class="text-foreground rounded-sm bg-yellow-200 px-0.5 dark:bg-yellow-500/40">{excerpt.match}</mark>{excerpt.after}
								</Item.Description>
							</Item.Content>
						</button>
					{/snippet}
				</Item.Root>
			{/each}
		</Item.Group>
	{/if}
</aside>
