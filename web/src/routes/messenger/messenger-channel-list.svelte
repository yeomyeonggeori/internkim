<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import CompassIcon from '@lucide/svelte/icons/compass';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import HashIcon from '@lucide/svelte/icons/hash';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import LockIcon from '@lucide/svelte/icons/lock';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { channelText } from '$lib/i18n/channel-text';
	import { conversationExportText } from '$lib/i18n/conversation-export-text';
	import { messengerInboxText } from '$lib/i18n/messenger-inbox-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import ConversationMenu from '$lib/components/channel/conversation-menu.svelte';
	import MessengerChannelSection from './messenger-channel-section.svelte';
	import MessengerListSkeleton from './messenger-list-skeleton.svelte';
	import { Button } from '$lib/components/ui/button';
	import { isSectionOpen, setSectionOpen } from './channel-sections.svelte';
	import type { ChannelSummary } from '$lib/components/channel/channel-api';
	import { conversationNameClass, unreadBadgeLabel } from '$lib/messenger/unread-badge';
	import { avatarPresenceOf } from '$lib/messenger/member-presence.svelte';
	import { circleChannels } from '$lib/messenger/circle-channels.svelte';
	import { pinnedChannelsOf } from './channel-order';
	import { noticeWhenChannelOrderFails } from './channel-order-notice';
	import type { ChannelOrderState } from './channel-order-state.svelte';
	import { ChannelListReorder, type ChannelListSection } from './channel-list-reorder.svelte';

	let {
		activeID,
		inbox,
		isLoading = false,
		error = '',
		retry,
		class: className = '',
		directMessages,
		groupChannels,
		muted,
		switchMuted,
		exportConversation,
		leaveChannel,
		hideDirectMessage,
		openNewDirectMessage,
		openNewChannel,
		openBrowseChannels,
		openOnPlatform,
		channelOrder,
		selectChannel,
		sidebarWidth = '16rem'
	}: {
		activeID: string | undefined;
		inbox?: { isActive: boolean; unreadCount: number; open: () => void };
		isLoading?: boolean;
		error?: string;
		retry?: () => void;
		class?: string;
		directMessages: ChannelSummary[];
		groupChannels: ChannelSummary[];
		muted: Set<string>;
		switchMuted: (conversationID: string) => void;
		exportConversation: (conversation: ChannelSummary) => void;
		leaveChannel: (channel: ChannelSummary) => void;
		hideDirectMessage?: (conversation: ChannelSummary) => void;
		openNewDirectMessage: () => void;
		openNewChannel?: () => void;
		openBrowseChannels?: () => void;
		openOnPlatform: { url: string; label: string } | null;
		channelOrder: ChannelOrderState;
		selectChannel: (channelID: string) => void;
		sidebarWidth?: string;
	} = $props();

	const text = createPageText(channelText);
	const exportText = createPageText(conversationExportText);
	const inboxText = createPageText(messengerInboxText);
	const shownActiveID = $derived(inbox?.isActive ? undefined : activeID);
	const pinnedChannels = $derived(pinnedChannelsOf(groupChannels, channelOrder.pinnedChannels));
	const unpinnedChannels = $derived(groupChannels.filter((channel) => !channelOrder.isPinned(channel.id)));
	const reorder = new ChannelListReorder(dropChannel);

	function dropChannel(section: ChannelListSection, draggedID: string, targetID: string) {
		const moved =
			section === 'pinned'
				? channelOrder.movePinned(draggedID, targetID)
				: channelOrder.moveChannel(groupChannels.map((channel) => channel.id), draggedID, targetID);
		noticeWhenChannelOrderFails(moved, text.reorderFailed);
	}

	function switchPinned(channel: ChannelSummary) {
		const notice = channelOrder.isPinned(channel.id) ? text.unpinFailed : text.pinFailed;
		noticeWhenChannelOrderFails(channelOrder.switchPinned(channel.id), notice);
	}
</script>

{#snippet channelRow(channel: ChannelSummary, section: ChannelListSection)}
	<ConversationMenu
		isMuted={muted.has(channel.id)}
		muteLabel={text.muteConversation}
		unmuteLabel={text.unmuteConversation}
		menuLabel={text.conversationMenu}
		onSwitchMuted={() => switchMuted(channel.id)}
		isPinned={section === 'pinned'}
		pinLabel={text.pinChannel}
		unpinLabel={text.unpinChannel}
		onSwitchPinned={() => switchPinned(channel)}
		leaveLabel={text.leaveChannel}
		onLeave={circleChannels.circleNameOf(channel.id) ? undefined : () => leaveChannel(channel)}
		itemProps={reorder.rowProps(section, channel.id)}
		holdMoves={reorder.holdMoves(section, channel.id)}
	>
		<Sidebar.MenuButton isActive={shownActiveID === channel.id} onclick={() => selectChannel(channel.id)}>
			{#if channel.isPrivate}<LockIcon />{:else}<HashIcon />{/if}
			<span class={conversationNameClass(channel.unreadCount, muted.has(channel.id), shownActiveID === channel.id)}>{channel.name}</span>
		</Sidebar.MenuButton>
		{@render unreadBadge(channel.unreadCount, true)}
	</ConversationMenu>
{/snippet}

{#snippet unreadBadge(unreadCount: number | undefined, givesWayToMenu: boolean)}
	{@const label = unreadBadgeLabel(unreadCount)}
	{#if label}
		<Sidebar.MenuBadge
			class={givesWayToMenu
				? 'md:group-hover/menu-item:opacity-0 md:group-focus-within/menu-item:opacity-0 md:group-has-[[data-sidebar=menu-action][data-state=open]]/menu-item:opacity-0'
				: undefined}
		>
			<Badge variant="secondary">
				<span class="sr-only">{text.unreadMessages}</span>
				{label}
			</Badge>
		</Sidebar.MenuBadge>
	{/if}
{/snippet}

<Sidebar.Provider class={cn('messenger-channel-list h-full min-h-0 w-auto', className)} style="--sidebar-width: {sidebarWidth};">
	<Sidebar.Root collapsible="none">
		<Sidebar.Content class="pt-2">
			{#if error}
				<div class="space-y-2 px-3 py-2"><p role="alert" class="text-sm text-destructive">{error}</p>{#if retry}<Button variant="outline" size="sm" onclick={retry}>{text.retry}</Button>{/if}</div>
			{/if}
			{#if inbox}
			<Sidebar.Group class="pb-0">
				<Sidebar.Menu>
					<Sidebar.MenuItem>
						<Sidebar.MenuButton isActive={inbox.isActive} onclick={inbox.open}>
							<InboxIcon />
							<span class={conversationNameClass(inbox.unreadCount, false, inbox.isActive)}>{inboxText.inbox}</span>
						</Sidebar.MenuButton>
						{@render unreadBadge(inbox.unreadCount, false)}
					</Sidebar.MenuItem>
				</Sidebar.Menu>
			</Sidebar.Group>
			{/if}
			{#if pinnedChannels.length > 0}
				<MessengerChannelSection
					title={text.pinnedChannelsTitle}
					open={isSectionOpen('pinned')}
					onOpenChange={(open) => setSectionOpen('pinned', open)}
				>
					<Sidebar.Menu>
						{#each pinnedChannels as channel (channel.id)}
							{@render channelRow(channel, 'pinned')}
						{/each}
					</Sidebar.Menu>
				</MessengerChannelSection>
			{/if}
			<MessengerChannelSection
				title={text.channelListTitle}
				open={isSectionOpen('channels')}
				onOpenChange={(open) => setSectionOpen('channels', open)}
			>
				{#snippet action()}
					{#if openNewChannel && openBrowseChannels}
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Sidebar.GroupAction {...props} aria-label={text.channelActions}>
										<PlusIcon />
									</Sidebar.GroupAction>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="end">
								<DropdownMenu.Item onclick={openNewChannel}>
									<PlusIcon />
									{text.newChannel}
								</DropdownMenu.Item>
								<DropdownMenu.Item onclick={openBrowseChannels}>
									<CompassIcon />
									{text.browseChannels}
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					{/if}
				{/snippet}
				<Sidebar.Menu>
					{#if isLoading}<li><MessengerListSkeleton label={text.loadingConversations} compact /></li>{/if}
					{#each unpinnedChannels as channel (channel.id)}
						{@render channelRow(channel, 'channels')}
					{/each}
				</Sidebar.Menu>
			</MessengerChannelSection>
			<MessengerChannelSection
				title={text.directMessagesTitle}
				open={isSectionOpen('directMessages')}
				onOpenChange={(open) => setSectionOpen('directMessages', open)}
			>
				{#snippet action()}
					<Sidebar.GroupAction aria-label={text.newDirectMessage} onclick={openNewDirectMessage}>
						<PlusIcon />
					</Sidebar.GroupAction>
				{/snippet}
				<Sidebar.Menu>
					{#if isLoading}<li><MessengerListSkeleton label={text.loadingConversations} compact /></li>{/if}
					{#each directMessages as conversation (conversation.id)}
						<ConversationMenu
							isMuted={muted.has(conversation.id)}
							muteLabel={text.muteConversation}
							unmuteLabel={text.unmuteConversation}
							menuLabel={text.conversationMenu}
							onSwitchMuted={() => switchMuted(conversation.id)}
							exportLabel={exportText.exportConversation}
							onExport={() => exportConversation(conversation)}
							hideLabel={text.hideConversation}
							onHide={hideDirectMessage ? () => hideDirectMessage(conversation) : undefined}
						>
							<Sidebar.MenuButton
								isActive={shownActiveID === conversation.id}
								onclick={() => selectChannel(conversation.id)}
							>
								<PersonAvatar
									name={conversation.name}
									seed={conversation.id}
									image={conversation.avatarURL ?? ''}
									memberID={conversation.counterpart?.memberID ?? ''}
									externalID={conversation.counterpart?.externalID ?? ''}
									{...avatarPresenceOf(conversation.counterpart?.memberID, text)}
									class="size-4"
								/>

								<span class={conversationNameClass(conversation.unreadCount, muted.has(conversation.id), shownActiveID === conversation.id)}>{conversation.name}</span>
							</Sidebar.MenuButton>
							{@render unreadBadge(conversation.unreadCount, true)}
						</ConversationMenu>
					{/each}
				</Sidebar.Menu>
			</MessengerChannelSection>
		</Sidebar.Content>
		{#if openOnPlatform}
			<Sidebar.Footer>
				<Sidebar.Menu>
					<Sidebar.MenuItem>
						<Sidebar.MenuButton>
							{#snippet child({ props })}
								<a
									{...props}
									href={openOnPlatform.url}
									target="_blank"
									rel="noreferrer noopener"
								>
									<ExternalLinkIcon />
									<span>{openOnPlatform.label}</span>
								</a>
							{/snippet}
						</Sidebar.MenuButton>
					</Sidebar.MenuItem>
				</Sidebar.Menu>
			</Sidebar.Footer>
		{/if}
	</Sidebar.Root>
</Sidebar.Provider>
