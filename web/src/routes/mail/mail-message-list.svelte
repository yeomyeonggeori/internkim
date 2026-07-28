<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import MailMessageRow from './mail-message-row.svelte';
	import MailOpenIcon from '@lucide/svelte/icons/mail-open';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import type { MailAccount, MailMessage } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		selectedMailboxCountText: string;
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
		text: (typeof mailText)['ko'];
		openSettings: () => void;
		loadMoreMessages: () => void | Promise<void>;
		setUnreadOnly: (isUnreadOnly: boolean) => void | Promise<void>;
		selectMessage: (message: MailMessage) => void;
	};

	let {
		account,
		selectedMailboxCountText,
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
		text,
		openSettings,
		loadMoreMessages,
		setUnreadOnly,
		selectMessage
	}: Props = $props();

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

<section class="flex min-h-0 flex-col border-r bg-muted/20 max-md:border-r-0">
	<div class="border-b bg-background p-3">
		<div class="flex items-center justify-between gap-2">
			{#if hasMailboxTrigger}
				<Sidebar.Trigger class="-ml-1" />
			{/if}
			<p class="min-w-0 flex-1 truncate text-xs text-muted-foreground">{hasLoadedAccount ? (account.isConfigured ? selectedMailboxCountText : text.connectAccount) : text.checkingMail}</p>
			<div class="flex shrink-0 items-center gap-2">
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
				<MailMessageRow {message} isActive={messageKey(selectedMessage) === messageKey(message)} {text} {selectMessage} />
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

			{#if canLoadMoreMessages}
				<div use:autoLoadMoreOnReach class="py-2">
					<Button variant="outline" size="sm" class="w-full" onclick={loadMoreMessages} disabled={isLoadingMessages}>
						{isLoadingMessages ? text.loadingMessages : text.loadMoreMessages}
					</Button>
				</div>
			{/if}
		</div>
	</div>
</section>
