<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import HashIcon from '@lucide/svelte/icons/hash';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import type { ChannelSummary } from '$lib/components/channel/channel-api';

	let {
		activeID,
		class: className = '',
		directMessages,
		groupChannels,
		openNewDirectMessage,
		openOnPlatform,
		reorderChannels,
		selectChannel,
		sidebarWidth = '16rem'
	}: {
		activeID: string | undefined;
		class?: string;
		directMessages: ChannelSummary[];
		groupChannels: ChannelSummary[];
		openNewDirectMessage: () => void;
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

<Sidebar.Provider class={cn('h-full min-h-0 w-auto', className)} style="--sidebar-width: {sidebarWidth};">
	<Sidebar.Root collapsible="none">
		<Sidebar.Content class="pt-2">
			<Sidebar.Group>
				<Sidebar.GroupLabel>{text.channelListTitle}</Sidebar.GroupLabel>
				<Sidebar.GroupContent>
					<Sidebar.Menu>
						{#each groupChannels as channel (channel.id)}
							<Sidebar.MenuItem
								draggable="true"
								class={dragOverChannelID === channel.id
									? 'border-primary rounded-md border'
									: 'rounded-md border border-transparent'}
								ondragstart={() => (draggedChannelID = channel.id)}
								ondragend={() => ((draggedChannelID = null), (dragOverChannelID = null))}
								ondragover={(event: DragEvent) => {
									if (!draggedChannelID || draggedChannelID === channel.id) return;
									event.preventDefault();
									dragOverChannelID = channel.id;
								}}
								ondragleave={() => {
									if (dragOverChannelID === channel.id) dragOverChannelID = null;
								}}
								ondrop={(event: DragEvent) => {
									event.preventDefault();
									dragOverChannelID = null;
									handleDrop(channel.id);
								}}
							>
								<Sidebar.MenuButton
									isActive={activeID === channel.id}
									onclick={() => selectChannel(channel.id)}
								>
									<HashIcon />
									<span>{channel.name}</span>
								</Sidebar.MenuButton>
							</Sidebar.MenuItem>
						{/each}
					</Sidebar.Menu>
				</Sidebar.GroupContent>
			</Sidebar.Group>
			<Sidebar.Group>
				<Sidebar.GroupLabel>{text.directMessagesTitle}</Sidebar.GroupLabel>
				<Sidebar.GroupAction aria-label={text.newDirectMessage} onclick={openNewDirectMessage}>
					<PlusIcon />
				</Sidebar.GroupAction>
				<Sidebar.GroupContent>
					<Sidebar.Menu>
						{#each directMessages as conversation (conversation.id)}
							<Sidebar.MenuItem>
								<Sidebar.MenuButton
									isActive={activeID === conversation.id}
									onclick={() => selectChannel(conversation.id)}
								>
									<PersonAvatar
										name={conversation.name}
										seed={conversation.id}
										image={conversation.avatarURL ?? ''}
										class="size-4"
									/>
									<span>{conversation.name}</span>
								</Sidebar.MenuButton>
							</Sidebar.MenuItem>
						{/each}
					</Sidebar.Menu>
				</Sidebar.GroupContent>
			</Sidebar.Group>
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
