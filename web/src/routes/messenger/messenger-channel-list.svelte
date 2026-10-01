<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import CompassIcon from '@lucide/svelte/icons/compass';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import HashIcon from '@lucide/svelte/icons/hash';
	import LockIcon from '@lucide/svelte/icons/lock';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import ConversationMenu from '$lib/components/channel/conversation-menu.svelte';
	import MessengerChannelSection from './messenger-channel-section.svelte';
	import { isSectionOpen, setSectionOpen } from './channel-sections.svelte';
	import type { ChannelSummary } from '$lib/components/channel/channel-api';
	import { isUnreadEmphasized, unreadBadgeLabel } from '$lib/messenger/unread-badge';
	import { avatarPresenceOf } from '$lib/messenger/member-presence.svelte';

	let {
		activeID,
		class: className = '',
		directMessages,
		groupChannels,
		muted,
		switchMuted,
		openNewDirectMessage,
		openNewChannel,
		openBrowseChannels,
		openOnPlatform,
		reorderChannels,
		selectChannel,
		sidebarWidth = '16rem'
	}: {
		activeID: string | undefined;
		class?: string;
		directMessages: ChannelSummary[];
		groupChannels: ChannelSummary[];
		muted: Set<string>;
		switchMuted: (conversationID: string) => void;
		openNewDirectMessage: () => void;
		openNewChannel?: () => void;
		openBrowseChannels?: () => void;
		openOnPlatform: { url: string; label: string } | null;
		reorderChannels: (draggedChannelID: string, targetChannelID: string) => void;
		selectChannel: (channelID: string) => void;
		sidebarWidth?: string;
	} = $props();

	const text = createPageText(channelText);

	let draggedChannelID = $state<string | null>(null);
	let dragOverChannelID = $state<string | null>(null);

	function handleDrop(targetChannelID: string) {
		if (!draggedChannelID) return;
		reorderChannels(draggedChannelID, targetChannelID);
		draggedChannelID = null;
	}
</script>

{#snippet unreadBadge(conversation: ChannelSummary)}
	{@const label = unreadBadgeLabel(conversation.unreadCount)}
	{#if label}
		<Sidebar.MenuBadge>
			<span class="sr-only">{text.unreadMessages}</span>
			{label}
		</Sidebar.MenuBadge>
	{/if}
{/snippet}

<Sidebar.Provider class={cn('h-full min-h-0 w-auto', className)} style="--sidebar-width: {sidebarWidth};">
	<Sidebar.Root collapsible="none">
		<Sidebar.Content class="pt-2">
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
					{#each groupChannels as channel (channel.id)}
						<ConversationMenu
							isMuted={muted.has(channel.id)}
							muteLabel={text.muteConversation}
							unmuteLabel={text.unmuteConversation}
							menuLabel={text.conversationMenu}
							onSwitchMuted={() => switchMuted(channel.id)}
							itemProps={{
								draggable: 'true',
								class:
									dragOverChannelID === channel.id
										? 'border-primary rounded-md border'
										: 'rounded-md border border-transparent',
								ondragstart: () => (draggedChannelID = channel.id),
								ondragend: () => ((draggedChannelID = null), (dragOverChannelID = null)),
								ondragover: (event: DragEvent) => {
									if (!draggedChannelID || draggedChannelID === channel.id) return;
									event.preventDefault();
									dragOverChannelID = channel.id;
								},
								ondragleave: () => {
									if (dragOverChannelID === channel.id) dragOverChannelID = null;
								},
								ondrop: (event: DragEvent) => {
									event.preventDefault();
									dragOverChannelID = null;
									handleDrop(channel.id);
								}
							}}
						>
							<Sidebar.MenuButton
								isActive={activeID === channel.id}
								onclick={() => selectChannel(channel.id)}
							>
								{#if channel.isPrivate}<LockIcon />{:else}<HashIcon />{/if}
								<span class={isUnreadEmphasized(channel.unreadCount, muted.has(channel.id)) ? 'font-semibold' : ''}>{channel.name}</span>
							</Sidebar.MenuButton>
							{@render unreadBadge(channel)}
						</ConversationMenu>
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
					{#each directMessages as conversation (conversation.id)}
						<ConversationMenu
							isMuted={muted.has(conversation.id)}
							muteLabel={text.muteConversation}
							unmuteLabel={text.unmuteConversation}
							menuLabel={text.conversationMenu}
							onSwitchMuted={() => switchMuted(conversation.id)}
						>
							<Sidebar.MenuButton
								isActive={activeID === conversation.id}
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

								<span class={isUnreadEmphasized(conversation.unreadCount, muted.has(conversation.id)) ? 'font-semibold' : ''}>{conversation.name}</span>
							</Sidebar.MenuButton>
							{@render unreadBadge(conversation)}
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
