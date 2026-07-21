<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import SendIcon from '@lucide/svelte/icons/send';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { mailboxUnreadCount, mailboxUnreadCountText } from './mail-page-utils';
	import type { MailAccount, Mailbox } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		mailboxes: Mailbox[];
		selectedMailbox: string;
		hasLoadedAccount: boolean;
		isLoadingMailboxes: boolean;
		text: (typeof mailText)['ko'];
		openSettings: () => void;
		openCompose: () => void;
		selectMailbox: (mailboxName: string) => void;
	};

	let { account, mailboxes, selectedMailbox, hasLoadedAccount, isLoadingMailboxes, text, openSettings, openCompose, selectMailbox }: Props = $props();

	function mailboxIcon(mailboxName: string) {
		const normalizedMailboxName = mailboxName.toLowerCase();
		if (normalizedMailboxName.includes('sent')) return SendIcon;
		if (normalizedMailboxName.includes('draft')) return FileTextIcon;
		if (normalizedMailboxName.includes('archive')) return ArchiveIcon;
		if (normalizedMailboxName.includes('trash')) return Trash2Icon;
		return InboxIcon;
	}
</script>

<aside class="flex min-h-0 flex-col border-r bg-sidebar text-sidebar-foreground max-md:hidden">
	<div class="flex h-14 items-center gap-2 border-b px-3">
		<div class="flex size-8 items-center justify-center rounded-md border bg-background">
			<MailIcon class="size-4" />
		</div>
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm font-medium">{text.pageName}</p>
			<p class="truncate text-xs text-muted-foreground">{hasLoadedAccount ? account.email || text.transportLabel : text.checkingMail}</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.settings} onclick={() => openSettings()}>
			<SettingsIcon />
		</Button>
	</div>

	<div class="border-b p-3">
		<Button class="h-9 w-full justify-start gap-2" variant="secondary" onclick={openCompose} disabled={!hasLoadedAccount || !account.isConfigured}>
			<PencilIcon />
			{text.compose}
		</Button>
	</div>

	<nav class="min-h-0 flex-1 overflow-auto p-2">
		<p class="px-2 py-2 text-xs font-medium text-muted-foreground">{text.mailboxes}</p>
		<div class="space-y-1">
			{#if !hasLoadedAccount}
				<p class="px-2 py-2 text-xs text-muted-foreground">{text.checkingMail}</p>
			{:else if isLoadingMailboxes && !mailboxes.length}
				<p class="px-2 py-2 text-xs text-muted-foreground">{text.loadingMailboxes}</p>
			{/if}
			{#each mailboxes as mailbox (mailbox.name)}
				{@const Icon = mailboxIcon(mailbox.name)}
				{@const unreadCount = mailboxUnreadCount(mailbox)}
				<button
					type="button"
					class="flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50"
					data-active={mailbox.name === selectedMailbox}
					disabled={!hasLoadedAccount || !account.isConfigured}
					onclick={() => selectMailbox(mailbox.name)}
				>
					<Icon class="size-4 shrink-0" />
					<span class="min-w-0 flex-1 truncate">{mailbox.displayName || mailbox.name}</span>
					{#if unreadCount}
						<span class="flex w-7 shrink-0 justify-end">
							<span
								class="inline-flex min-w-5 items-center justify-center rounded-md bg-primary px-1.5 py-0.5 text-[10px] font-medium leading-none text-primary-foreground"
								title={mailboxUnreadCountText(mailbox, text)}
								aria-label={mailboxUnreadCountText(mailbox, text)}
							>
								{unreadCount}
							</span>
						</span>
					{/if}
				</button>
			{/each}
		</div>
	</nav>

	<div class="border-t p-3">
		<button type="button" class="w-full rounded-lg border bg-background p-3 text-left" onclick={() => openSettings()}>
			<p class="text-xs font-medium">{hasLoadedAccount ? (account.isConfigured ? text.connectedAccount : text.noAccountConnected) : text.checkingMail}</p>
			<p class="mt-1 text-xs leading-5 text-muted-foreground">
				{hasLoadedAccount ? (account.isConfigured ? text.connectedAccountDescription : text.noAccountConnectedDescription) : text.checkingMailDescription}
			</p>
		</button>
	</div>
</aside>
