<script lang="ts">
	import Channel from '$lib/components/channel/channel.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import HashIcon from '@lucide/svelte/icons/hash';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		ensureDirectMessage,
		fetchConversations,
		fetchPeople,
		type ChannelSummary,
		type Person
	} from '$lib/components/channel/channel-api';
	import { onMount } from 'svelte';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';
	import { syncMattermostToBuzz } from '$lib/buzz-mm-sync';

	const text = createPageText(channelText);

	let conversations = $state<ChannelSummary[]>([]);
	let activeID = $state<string | undefined>(undefined);
	let isNewDirectMessageOpen = $state(false);
	let people = $state<Person[]>([]);
	let hasSyncedMattermost = false;

	async function loadConversationList() {
		conversations = await fetchConversations();
	}

	$effect(() => {
		if (!buzzIdentity.secretHex || hasSyncedMattermost) return;
		hasSyncedMattermost = true;
		syncMattermostToBuzz(buzzIdentity.secretHex)
			.then(() => loadConversationList())
			.catch(() => {});
	});

	async function openNewDirectMessage() {
		isNewDirectMessageOpen = true;
		try {
			people = await fetchPeople();
		} catch {
			people = [];
		}
	}

	async function startDirectMessage(person: Person) {
		isNewDirectMessageOpen = false;
		try {
			const channelID = await ensureDirectMessage(person.id);
			if (!channelID) return;
			await loadConversationList();
			activeID = channelID;
		} catch {
			// keep the current conversation on failure
		}
	}

	const groupChannels = $derived(conversations.filter((conversation) => conversation.kind === 'group'));
	const directMessages = $derived(conversations.filter((conversation) => conversation.kind === 'dm'));
	const activeConversation = $derived(
		conversations.find((conversation) => conversation.id === activeID)
	);

	onMount(async () => {
		try {
			await loadConversationList();
			const preferredDirectMessage = conversations.find((conversation) => conversation.kind === 'dm');
			activeID = (preferredDirectMessage ?? conversations[0])?.id;
		} catch {
			conversations = [];
		}
	});
</script>

<svelte:head>
	<title>{text.messenger}</title>
</svelte:head>


<div class="flex min-h-0 w-full flex-1 p-4">
	<Card.Root class="flex min-h-0 w-full flex-row gap-0 overflow-hidden p-0">
		<Sidebar.Provider class="h-full min-h-0 w-auto" style="--sidebar-width: 16rem;">
			<Sidebar.Root collapsible="none" class="border-r">
				<Sidebar.Content class="pt-2">
					<Sidebar.Group>
						<Sidebar.GroupLabel>{text.channelListTitle}</Sidebar.GroupLabel>
						<Sidebar.GroupContent>
							<Sidebar.Menu>
								{#each groupChannels as channel (channel.id)}
									<Sidebar.MenuItem>
										<Sidebar.MenuButton
											isActive={activeID === channel.id}
											onclick={() => (activeID = channel.id)}
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
											onclick={() => (activeID = conversation.id)}
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
			</Sidebar.Root>
		</Sidebar.Provider>
		<div class="flex min-h-0 min-w-0 flex-1 flex-col">
			<header class="flex h-14 shrink-0 items-center gap-2 border-b px-6">
				{#if activeConversation?.kind === 'dm'}
					<PersonAvatar
						name={activeConversation.name}
						seed={activeConversation.id}
						image={activeConversation.avatarURL ?? ''}
						class="size-6"
					/>
				{:else if activeConversation}
					<HashIcon class="text-muted-foreground size-5 shrink-0" />
				{/if}
				<span class="font-semibold">{activeConversation?.name ?? text.messenger}</span>
			</header>
			{#key activeID}
				<Channel channelId={activeID} />
			{/key}
		</div>
	</Card.Root>
</div>

<Dialog.Root bind:open={isNewDirectMessageOpen}>
	<Dialog.Content class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>{text.newDirectMessage}</Dialog.Title>
		</Dialog.Header>
		<div class="-mx-2 max-h-80 overflow-y-auto">
			{#each people as person (person.id)}
				<button
					type="button"
					class="hover:bg-muted/60 flex w-full items-center gap-3 rounded-md px-2 py-2 text-left"
					onclick={() => startDirectMessage(person)}
				>
					<PersonAvatar
						name={person.name}
						seed={person.id}
						image={person.avatarURL ?? ''}
						class="size-8"
					/>
					<span class="truncate text-sm font-medium">{person.name}</span>
				</button>
			{/each}
			{#if people.length === 0}
				<p class="text-muted-foreground px-2 py-6 text-center text-sm">{text.noPeople}</p>
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>
