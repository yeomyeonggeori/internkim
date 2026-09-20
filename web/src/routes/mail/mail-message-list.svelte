<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Input } from '$lib/components/ui/input';
	import * as Tabs from '$lib/components/ui/tabs';
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
	<div class="flex h-[52px] shrink-0 items-center gap-2 border-b px-4">
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

	<div class="shrink-0 bg-background/95 p-4 backdrop-blur supports-[backdrop-filter]:bg-background/60">
		<form onsubmit={submitSearch}>
			<div class="relative">
				<SearchIcon class="absolute left-2 top-[50%] size-4 translate-y-[-50%] text-muted-foreground" />
				<Input placeholder={text.searchMail} class="pl-8" bind:value={searchText} />
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

		{#if !messages.length && !(isLoadingMessages && account.isConfigured)}
			<div class="flex flex-1 flex-col justify-center px-3 text-center">
				<MailOpenIcon class="mx-auto size-6 text-muted-foreground/60" />
				{#if !hasLoadedAccount}
					<p class="mt-3 text-sm font-medium">{text.checkingMail}</p>
					<p class="mt-1 text-xs text-muted-foreground">{text.checkingMailDescription}</p>
				{:else if account.isConfigured}
					<p class="mt-3 text-sm font-medium">{text.noMessages}</p>
					<p class="mt-1 text-xs text-muted-foreground">{text.emptyMailbox}</p>
				{:else}
					<p class="mt-3 text-sm font-medium">{text.connectMail}</p>
					<p class="mt-1 text-xs text-muted-foreground">{text.emptyUnconfigured}</p>
					<Button class="mt-4 gap-2 self-center" variant="secondary" onclick={() => openSettings()}>
						<SettingsIcon />
						{text.connectAccount}
					</Button>
				{/if}
			</div>
		{:else}
			<div class="flex flex-col gap-2">
				{#if !messages.length}
					{#each Array.from({ length: 6 }) as _, placeholderIndex (placeholderIndex)}
						<MailMessageRow isPlaceholder {text} />
					{/each}
				{/if}
				{#each messages as message (messageKey(message))}
					<MailMessageRow {message} isActive={messageKey(selectedMessage) === messageKey(message)} {text} {selectMessage} />
				{/each}

				{#if canLoadMoreMessages}
					<div use:autoLoadMoreOnReach class="py-2">
						<Button variant="outline" size="sm" class="w-full" onclick={loadMoreMessages} disabled={isLoadingMessages}>
							{isLoadingMessages ? text.loadingMessages : text.loadMoreMessages}
						</Button>
					</div>
				{/if}
			</div>
		{/if}
	</div>
</section>
