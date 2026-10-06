<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import AtSignIcon from '@lucide/svelte/icons/at-sign';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import SendIcon from '@lucide/svelte/icons/send';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { mailboxUnreadCount, mailboxUnreadCountText } from './mail-page-utils';
	import type { MailAccount, Mailbox } from './mail-types';
	import type { mailText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type Props = {
		account: MailAccount;
		mailboxes: Mailbox[];
		selectedMailbox: string;
		hasLoadedAccount: boolean;
		isLoadingMailboxes: boolean;
		text: PageText<typeof mailText>;
		openSettings: () => void;
		selectMailbox: (mailboxName: string) => void;
	};

	let { account, mailboxes, selectedMailbox, hasLoadedAccount, isLoadingMailboxes, text, openSettings, selectMailbox }: Props = $props();

	function accountText() {
		if (!hasLoadedAccount) return text.checkingMail;
		if (!account.isConfigured) return text.accountDisconnected;
		return account.displayName || account.email || text.transportLabel;
	}

	function mailboxLabel(mailbox: Mailbox) {
		return mailbox.displayName || mailbox.name;
	}

	function mailboxIcon(mailboxName: string) {
		const normalizedMailboxName = mailboxName.toLowerCase();
		if (normalizedMailboxName.includes('sent')) return SendIcon;
		if (normalizedMailboxName.includes('draft')) return FileTextIcon;
		if (normalizedMailboxName.includes('archive')) return ArchiveIcon;
		if (normalizedMailboxName.includes('trash')) return Trash2Icon;
		return InboxIcon;
	}
</script>

<Sidebar.Root collapsible="icon" class="absolute h-full">
	<Sidebar.Header class="h-[52px] justify-center border-b p-2">
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<Sidebar.MenuButton variant="outline" tooltipContent={accountText()} onclick={() => openSettings()}>
					<AtSignIcon />
					<span>{accountText()}</span>
				</Sidebar.MenuButton>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Header>

	<Sidebar.Content>
		<Sidebar.Group>
			<Sidebar.GroupLabel>{text.mailboxes}</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				{#if !hasLoadedAccount || (isLoadingMailboxes && !mailboxes.length)}
					<div role="status" aria-label={text.loadingMailboxes} aria-busy="true" class="grid gap-2 p-2">
						{#each [0, 1, 2, 3] as row (row)}<div aria-hidden="true" class="flex items-center gap-2"><Skeleton class="size-4 shrink-0" /><Skeleton class="h-5 w-3/4 group-data-[collapsible=icon]:hidden" /></div>{/each}
					</div>
				{:else}
				<Sidebar.Menu>
					{#each mailboxes as mailbox (mailbox.name)}
						{@const Icon = mailboxIcon(mailbox.name)}
						{@const unreadCount = mailboxUnreadCount(mailbox)}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton
								isActive={mailbox.name === selectedMailbox}
								tooltipContent={mailboxLabel(mailbox)}
								aria-disabled={!hasLoadedAccount || !account.isConfigured}
								onclick={() => selectMailbox(mailbox.name)}
							>
								<Icon />
								<span>{mailboxLabel(mailbox)}</span>
							</Sidebar.MenuButton>
							{#if unreadCount}
								<Sidebar.MenuBadge aria-label={mailboxUnreadCountText(mailbox, text)}>{unreadCount}</Sidebar.MenuBadge>
							{/if}
						</Sidebar.MenuItem>
					{/each}
				</Sidebar.Menu>
				{/if}
			</Sidebar.GroupContent>
		</Sidebar.Group>
	</Sidebar.Content>
</Sidebar.Root>
