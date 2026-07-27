<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MailOpenIcon from '@lucide/svelte/icons/mail-open';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';
	import SearchIcon from '@lucide/svelte/icons/search';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import type { MailAccount, MailMessage } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		selectedMailbox: string;
		selectedMailboxCountText: string;
		isLoading: boolean;
		isSyncing: boolean;
		hasLoadedAccount: boolean;
		isLoadingMessages: boolean;
		searchText: string;
		isUnreadOnly: boolean;
		messagePageIndex: number;
		canPreviousMessagePage: boolean;
		canNextMessagePage: boolean;
		errorMessage: string;
		messages: MailMessage[];
		selectedMessage: MailMessage | null;
		text: (typeof mailText)['ko'];
		openSettings: () => void;
		loadMessages: () => void | Promise<void>;
		loadPreviousMessages: () => void | Promise<void>;
		loadNextMessages: () => void | Promise<void>;
		setUnreadOnly: (isUnreadOnly: boolean) => void | Promise<void>;
		selectMessage: (message: MailMessage) => void;
	};

	let {
		account,
		selectedMailbox,
		selectedMailboxCountText,
		isLoading,
		isSyncing,
		hasLoadedAccount,
		isLoadingMessages,
		searchText = $bindable(''),
		isUnreadOnly,
		messagePageIndex,
		canPreviousMessagePage,
		canNextMessagePage,
		errorMessage,
		messages,
		selectedMessage,
		text,
		openSettings,
		loadMessages,
		loadPreviousMessages,
		loadNextMessages,
		setUnreadOnly,
		selectMessage
	}: Props = $props();

	function messageKey(message: MailMessage | null) {
		if (!message) return '';
		return `${message.mailbox}:${message.uid}`;
	}
</script>

<section class="flex min-h-0 flex-col border-r bg-muted/20 max-md:border-r-0">
	<header class="flex h-14 shrink-0 items-center gap-2 border-b bg-background px-3">
		<Button class="md:hidden" variant="ghost" size="icon-sm" aria-label={text.settings} onclick={() => openSettings()}>
			<PanelLeftIcon />
		</Button>
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm font-semibold">{selectedMailbox}</p>
			{#if selectedMailboxCountText}
				<p class="truncate text-xs text-muted-foreground">{selectedMailboxCountText}</p>
			{/if}
		</div>
	</header>

	<div class="border-b bg-background p-3">
		<div class="relative">
			<SearchIcon class="pointer-events-none absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
			<Input class="h-9 pl-8" bind:value={searchText} placeholder={text.search} disabled={!hasLoadedAccount || !account.isConfigured} onkeydown={(event) => event.key === 'Enter' && loadMessages()} />
		</div>
		<div class="mt-3 flex items-center justify-between">
			<p class="text-xs font-medium text-muted-foreground">{hasLoadedAccount ? (account.isConfigured ? selectedMailbox : text.connectAccount) : text.checkingMail}</p>
			<div class="flex items-center gap-2">
				<Switch id="mail-unread-only" checked={isUnreadOnly} onCheckedChange={setUnreadOnly} />
				<Label for="mail-unread-only" class={`text-xs ${isUnreadOnly ? 'font-semibold text-foreground' : 'text-muted-foreground'}`}>{text.unread}</Label>
			</div>
		</div>
	</div>

	<div class="min-h-0 flex-1 overflow-auto p-2">
		{#if errorMessage}
			<div role="alert" class="mb-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
				<p class="font-medium">{text.needsAttention}</p>
				<p class="mt-1 line-clamp-3 text-xs leading-5">{errorMessage}</p>
			</div>
		{/if}

		<div class="space-y-2">
			{#if messages.length && isLoadingMessages}
				<p class="rounded-md border bg-background px-3 py-2 text-xs text-muted-foreground">{text.loadingMessages}</p>
			{/if}
			{#each messages as message (messageKey(message))}
				<button
					type="button"
					class="w-full rounded-lg border bg-background p-3 text-left shadow-xs transition-colors hover:bg-muted/60 data-[active=true]:border-primary/40 data-[active=true]:bg-background data-[active=true]:shadow-sm"
					data-active={messageKey(selectedMessage) === messageKey(message)}
					onclick={() => selectMessage(message)}
				>
					<div class="flex items-center gap-2">
						{#if message.isRead}
							<MailOpenIcon class="size-4 text-muted-foreground" />
						{:else}
							<MailIcon class="size-4" />
						{/if}
						<span class="min-w-0 flex-1 truncate text-sm font-medium">{message.from || text.unknownSender}</span>
					</div>
					<p class="mt-2 truncate text-sm">{message.subject || text.noSubject}</p>
					{#if message.preview}
						<p class="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{message.preview}</p>
					{/if}
				</button>
			{/each}

			{#if !messages.length}
				<div class="rounded-lg border border-dashed bg-background p-8 text-center">
					<MailOpenIcon class="mx-auto size-5 text-muted-foreground" />
					{#if !hasLoadedAccount}
						<p class="mt-3 text-sm font-medium">{text.checkingMail}</p>
						<p class="mt-1 text-xs text-muted-foreground">{text.checkingMailDescription}</p>
					{:else if account.isConfigured && isLoadingMessages}
						<p class="mt-3 text-sm font-medium">{text.loadingMessages}</p>
						<p class="mt-1 text-xs text-muted-foreground">{text.checkingMailDescription}</p>
					{:else}
						<p class="mt-3 text-sm font-medium">{account.isConfigured ? text.noMessages : text.connectMail}</p>
						<p class="mt-1 text-xs text-muted-foreground">{account.isConfigured ? text.emptyMailbox : text.emptyUnconfigured}</p>
					{/if}
					{#if hasLoadedAccount && !account.isConfigured}
						<Button class="mt-4 gap-2" variant="secondary" onclick={() => openSettings()}>
							<SettingsIcon />
							{text.connectAccount}
						</Button>
					{/if}
				</div>
			{/if}

			{#if messages.length || canPreviousMessagePage || canNextMessagePage}
				<nav aria-label={text.messagePagination} class="flex items-center justify-between gap-2 py-2 text-xs text-muted-foreground">
					<Button variant="outline" size="sm" class="gap-1" onclick={loadPreviousMessages} disabled={!canPreviousMessagePage || isLoadingMessages}>
						<ChevronLeftIcon class="size-3.5" />
						{text.previousPage}
					</Button>
					<span class="tabular-nums">{text.messagePage.replace('{page}', String(messagePageIndex + 1))}</span>
					<Button variant="outline" size="sm" class="gap-1" onclick={loadNextMessages} disabled={!canNextMessagePage || isLoadingMessages}>
						{text.nextPage}
						<ChevronRightIcon class="size-3.5" />
					</Button>
				</nav>
			{/if}
		</div>
	</div>
</section>
