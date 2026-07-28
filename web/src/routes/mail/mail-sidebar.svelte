<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import AtSignIcon from '@lucide/svelte/icons/at-sign';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import SendIcon from '@lucide/svelte/icons/send';
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
	<Sidebar.Content>
		<Sidebar.Group>
			<Sidebar.GroupLabel>{text.mailboxes}</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				{#if !hasLoadedAccount}
					<p class="px-2 py-2 text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">{text.checkingMail}</p>
				{:else if isLoadingMailboxes && !mailboxes.length}
					<p class="px-2 py-2 text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">{text.loadingMailboxes}</p>
				{/if}
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
			</Sidebar.GroupContent>
		</Sidebar.Group>
	</Sidebar.Content>

	<Sidebar.Footer>
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<Sidebar.MenuButton class="text-muted-foreground" tooltipContent={accountText()} onclick={() => openSettings()}>
					<AtSignIcon />
					<span class="text-xs">{accountText()}</span>
				</Sidebar.MenuButton>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Footer>
</Sidebar.Root>
