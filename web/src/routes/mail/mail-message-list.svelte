<script lang="ts">
	import { Spinner } from '$lib/components/ui/spinner';
	import { Button } from '$lib/components/ui/button';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Input } from '$lib/components/ui/input';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Empty from '$lib/components/ui/empty';
	import MailMessageRow from './mail-message-row.svelte';
	import MailOpenIcon from '@lucide/svelte/icons/mail-open';
	import SearchIcon from '@lucide/svelte/icons/search';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import type { MailAccount, MailMessage } from './mail-types';
	import type { mailText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type Props = {
		account: MailAccount;
		selectedMailboxLabel: string;
		isLoading: boolean;
		isSyncing: boolean;
		hasLoadedAccount: boolean;
		isLoadingMessages: boolean;
		isUnreadOnly: boolean;
		canLoadMoreMessages: boolean;
		errorMessage: string;
		messages: MailMessage[];
		selectedMessage: MailMessage | null;
		hasMailboxTrigger: boolean;
		searchText: string;
		activeSearchText?: string;
		text: PageText<typeof mailText>;
		openSettings: () => void;
		loadMoreMessages: () => void | Promise<void>;
		setUnreadOnly: (isUnreadOnly: boolean) => void | Promise<void>;
		searchMessages: () => void | Promise<void>;
		selectMessage: (message: MailMessage) => void;
	};

	let {
		account,
		selectedMailboxLabel,
		isLoading,
		isSyncing,
		hasLoadedAccount,
		isLoadingMessages,
		isUnreadOnly,
		canLoadMoreMessages,
		errorMessage,
		messages,
		selectedMessage,
		hasMailboxTrigger,
		searchText = $bindable(''),
		activeSearchText = '',
		text,
		openSettings,
		loadMoreMessages,
		setUnreadOnly,
		searchMessages,
		selectMessage
	}: Props = $props();

	function submitSearch(event: SubmitEvent) {
		event.preventDefault();
		searchMessages();
	}

	async function clearFilters() {
		searchText = '';
		if (isUnreadOnly) await setUnreadOnly(false);
		if (activeSearchText) await searchMessages();
	}

	function autoLoadMoreOnReach(sentinel: HTMLElement) {
		const observer = new IntersectionObserver((entries) => {
			if (!entries.some((entry) => entry.isIntersecting)) return;
			if (!canLoadMoreMessages || isLoadingMessages) return;
			loadMoreMessages();
		});
		observer.observe(sentinel);
		return { destroy: () => observer.disconnect() };
	}

	function messageKey(message: MailMessage | null) {
		if (!message) return '';
		return `${message.mailbox}:${message.uid}`;
	}
</script>

<section class="flex min-h-0 flex-col border-r bg-background max-md:border-r-0">
	<div class="flex min-h-[52px] shrink-0 flex-wrap items-center gap-2 border-b px-3 py-1 sm:flex-nowrap sm:px-4">
		{#if hasMailboxTrigger}
			<Sidebar.Trigger class="-ml-1" />
		{/if}
		<h1 class="min-w-0 flex-1 truncate text-xl font-bold">{selectedMailboxLabel}</h1>
		<Tabs.Root value={isUnreadOnly ? 'unread' : 'all'} onValueChange={(value) => setUnreadOnly(value === 'unread')} class="shrink-0">
			<Tabs.List>
				<Tabs.Trigger value="all">{text.allMail}</Tabs.Trigger>
				<Tabs.Trigger value="unread">{text.unread}</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>
	</div>

	<div class="shrink-0 bg-background/95 p-3 backdrop-blur supports-[backdrop-filter]:bg-background/60 sm:p-4">
		<form onsubmit={submitSearch}>
			<div class="relative">
				<SearchIcon class="absolute left-2 top-[50%] size-4 translate-y-[-50%] text-muted-foreground" />
				<Input aria-label={text.searchMail} placeholder={text.searchMail} class="pl-8" bind:value={searchText} />
			</div>
		</form>
	</div>

	<div class="flex min-h-0 flex-1 flex-col gap-2 overflow-auto p-4">
		{#if errorMessage}
			<div role="alert" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
				<p class="font-medium">{text.needsAttention}</p>
				<p class="mt-1 line-clamp-3 text-xs leading-5">{errorMessage}</p>
			</div>
		{/if}

			{#if !messages.length && (isLoading || !hasLoadedAccount || (isLoadingMessages && account.isConfigured))}
				<div role="status" aria-label={text.loadingMessages} aria-busy="true" class="flex flex-col gap-2">
					{#each [0, 1, 2, 3, 4, 5] as row (row)}<MailMessageRow isPlaceholder {text} />{/each}
				</div>
			{:else if !messages.length && !errorMessage}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon"><MailOpenIcon /></Empty.Media>
					<Empty.Title>{!account.isConfigured ? text.connectMail : activeSearchText ? text.noSearchResults : isUnreadOnly ? text.noUnreadMessages : text.noMessages}</Empty.Title>
					<Empty.Description>{!account.isConfigured ? text.emptyUnconfigured : activeSearchText || isUnreadOnly ? text.emptySearch : text.emptyMailbox}</Empty.Description>
				</Empty.Header>
				{#if !account.isConfigured}
					<Empty.Content><Button variant="secondary" onclick={openSettings}><SettingsIcon />{text.connectAccount}</Button></Empty.Content>
				{:else if activeSearchText || isUnreadOnly}
					<Empty.Content><Button variant="outline" size="sm" onclick={clearFilters}>{text.resetFilters}</Button></Empty.Content>
				{/if}
			</Empty.Root>
		{:else if messages.length}
			<div class="flex flex-col gap-2">
				{#each messages as message (messageKey(message))}
					<MailMessageRow {message} isActive={messageKey(selectedMessage) === messageKey(message)} {text} {selectMessage} />
				{/each}

				{#if canLoadMoreMessages}
					<div use:autoLoadMoreOnReach class="py-2">
						<Button variant="outline" size="sm" class="w-full" onclick={loadMoreMessages} disabled={isLoadingMessages}>
							{#if isLoadingMessages}<Spinner />{/if}
							{isLoadingMessages ? text.loadingMessages : text.loadMoreMessages}
						</Button>
					</div>
				{/if}
			</div>
		{/if}
	</div>
</section>
