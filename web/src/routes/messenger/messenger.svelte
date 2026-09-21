<script lang="ts">
	import Channel from '$lib/components/channel/channel.svelte';
	import MessengerChannelList from './messenger-channel-list.svelte';
	import MessengerBrowseChannelsDialog from './messenger-browse-channels-dialog.svelte';
	import MessengerChannelDetails from './messenger-channel-details.svelte';
	import MessengerChannelHeaderActions from './messenger-channel-header-actions.svelte';
	import MessengerChannelMembersDialog from './messenger-channel-members-dialog.svelte';
	import MessengerChannelOwnerDialog from './messenger-channel-owner-dialog.svelte';
	import MessengerNewChannelDialog from './messenger-new-channel-dialog.svelte';
	import { muteConversation, mutedConversations, unmuteConversation } from '$lib/notifications/muted-conversations';
	import { toast } from 'svelte-sonner';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import HashIcon from '@lucide/svelte/icons/hash';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';
	import { channelText } from '$lib/i18n/channel-text';
	import { mentionPeopleOf } from '$lib/messenger/mention-candidates';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		ensureDirectMessage,
		ensureAgentConversation,
		fetchConversations,
		fetchPeople,
		type ChannelSummary,
		type Person
	} from '$lib/components/channel/channel-api';
	import { onDestroy, onMount } from 'svelte';
	import { onCompanyEvent } from '$lib/host-bridge';
	import type { CompanyEvent } from '$lib/company-event';
	import { isSupabaseConfigured } from '$lib/supabase';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { syncMattermostToBuzz } from '$lib/buzz-mm-sync';
	import { loadChannelOrder, saveChannelOrder, orderChannels, moveChannel } from './channel-order';
	import { holdBackNotificationsFor } from '$lib/native-shell/open-conversation';

	const lastChannelKey = 'messenger-last-channel';

	function loadLastChannelID(): string | null {
		if (typeof localStorage === 'undefined') return null;
		return localStorage.getItem(lastChannelKey);
	}

	function selectChannel(channelID: string) {
		activeID = channelID;
		if (typeof localStorage !== 'undefined') localStorage.setItem(lastChannelKey, channelID);
		const url = new URL(location.href);
		url.searchParams.set('channel', channelID);
		replaceState(url, {});
	}

	const text = createPageText(channelText);

	let conversations = $state<ChannelSummary[]>([]);
	let activeID = $state<string | undefined>(undefined);
	let isNewDirectMessageOpen = $state(false);
	let isNewChannelOpen = $state(false);
	let isBrowseChannelsOpen = $state(false);
	let isChannelDetailsOpen = $state(false);
	let isChannelMembersOpen = $state(false);
	let isChannelOwnerOpen = $state(false);
	let channelOwnerAction = $state<'add' | 'hand-over'>('add');
	const canManageChannels = isSupabaseConfigured();
	let isChannelSheetOpen = $state(false);
	let people = $state<Person[]>([]);
	let muted = $state<Set<string>>(new Set());
	let hasSyncedMattermost = false;
	let userChannelOrder = $state<string[]>([]);

	const isMobile = new IsMobile();

	$effect(() => {
		if (isMobile.current) return;
		isChannelSheetOpen = false;
	});

	function selectChannelFromSheet(channelID: string) {
		isChannelSheetOpen = false;
		selectChannel(channelID);
	}

	function openNewDirectMessageFromSheet() {
		isChannelSheetOpen = false;
		openNewDirectMessage();
	}

	function openNewChannel() {
		isChannelSheetOpen = false;
		isNewChannelOpen = true;
	}

	function openBrowseChannels() {
		isChannelSheetOpen = false;
		isBrowseChannelsOpen = true;
	}

	function openChannelMembers() {
		isChannelMembersOpen = true;
		refreshConversations('the channel list did not refresh for the member list');
	}

	function openChannelDetails() {
		isChannelDetailsOpen = true;
		refreshConversations('the channel list did not refresh for the channel details');
	}

	function refreshConversations(why: string) {
		loadConversationList().catch((failure: unknown) => console.warn(why, failure));
	}

	async function leftChannel(channelID: string) {
		conversations = conversations.filter((conversation) => conversation.id !== channelID);
		const next = groupChannels[0] ?? conversations[0];
		if (next) selectChannel(next.id);
		refreshConversations('the channel list did not refresh after leaving');
	}

	async function showChannel(channelID: string) {
		try {
			await loadConversationList();
		} catch (failure) {
			console.warn('the channel list did not refresh', failure);
		}
		selectChannel(channelID);
	}

	const conversationsCacheKey = 'messenger-conversations';

	function loadCachedConversations(): ChannelSummary[] {
		if (typeof sessionStorage === 'undefined') return [];
		try {
			const cached: unknown = JSON.parse(sessionStorage.getItem(conversationsCacheKey) ?? '[]');
			return Array.isArray(cached) ? (cached as ChannelSummary[]) : [];
		} catch {
			return [];
		}
	}

	async function loadConversationList() {
		conversations = await fetchConversations();
		if (typeof sessionStorage !== 'undefined') {
			sessionStorage.setItem(conversationsCacheKey, JSON.stringify(conversations));
		}
	}

	function selectInitialChannel() {
		const requestedID = page.url.searchParams.get('channel') ?? loadLastChannelID();
		const remembered = requestedID
			? conversations.find((conversation) => conversation.id === requestedID)
			: undefined;
		const initial = remembered ?? groupChannels[0] ?? conversations[0];
		if (initial) selectChannel(initial.id);
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
			selectChannel(channelID);
		} catch {
			return;
		}
	}

	async function startAgentConversation() {
		isNewDirectMessageOpen = false;
		try {
			const channelID = await ensureAgentConversation();
			await loadConversationList();
			selectChannel(channelID);
		} catch {
			toast.error(text.unavailableTitle);
		}
	}

	const groupChannels = $derived(
		orderChannels(
			conversations.filter((conversation) => conversation.kind === 'group'),
			userChannelOrder
		)
	);
	const directMessages = $derived(conversations.filter((conversation) => conversation.kind === 'dm'));

	function reorderChannels(draggedChannelID: string, targetChannelID: string) {
		const reordered = moveChannel(
			groupChannels.map((channel) => channel.id),
			draggedChannelID,
			targetChannelID
		);
		userChannelOrder = reordered;
		saveChannelOrder(reordered);
	}
	const activeConversation = $derived(
		conversations.find((conversation) => conversation.id === activeID)
	);

	function platformLabel(platform: string): string {
		return platform.charAt(0).toUpperCase() + platform.slice(1);
	}

	const openOnPlatform = $derived.by(() => {
		if (!activeConversation?.webURL || !activeConversation.platform) return null;
		return {
			url: activeConversation.webURL,
			label: text.openInPlatform.replace('{platform}', platformLabel(activeConversation.platform))
		};
	});

	$effect(() => {
		breadcrumbMeta.value = activeConversation?.name ?? '';
		return () => {
			breadcrumbMeta.value = '';
		};
	});

	async function switchMuted(conversationID: string) {
		const wasMuted = muted.has(conversationID);
		const next = new Set(muted);
		if (wasMuted) next.delete(conversationID);
		else next.add(conversationID);
		muted = next;
		try {
			if (wasMuted) await unmuteConversation(conversationID);
			else await muteConversation(conversationID);
		} catch (error) {
			const restored = new Set(muted);
			if (wasMuted) restored.add(conversationID);
			else restored.delete(conversationID);
			muted = restored;
			toast.error(error instanceof Error ? error.message : text.muteFailed);
		}
	}

	const conversationsNotMine = new Set<string>();
	let isReadingList: Promise<void> | null = null;

	function readListOnArrival(event: CompanyEvent): void {
		if (event.kind !== 'message.arrived' || !event.conversationID) return;
		const conversationID = event.conversationID;
		if (conversationsNotMine.has(conversationID)) return;
		if (conversations.some((conversation) => conversation.id === conversationID)) return;
		isReadingList ??= loadConversationList()
			.then(() => {
				if (!conversations.some((conversation) => conversation.id === conversationID)) {
					conversationsNotMine.add(conversationID);
				}
			})
			.catch(() => undefined)
			.finally(() => {
				isReadingList = null;
			});
	}

	let stopListeningForArrivals = () => {};

	$effect(() => {
		holdBackNotificationsFor(activeID).catch((failure: unknown) =>
			console.warn('the shell is not holding back this conversation', failure)
		);
	});

	onDestroy(() => {
		stopListeningForArrivals();
		holdBackNotificationsFor(undefined).catch((failure: unknown) =>
			console.warn('the shell is still holding back a closed conversation', failure)
		);
	});

	onMount(async () => {
		userChannelOrder = loadChannelOrder();
		if (isSupabaseConfigured()) stopListeningForArrivals = onCompanyEvent(readListOnArrival);

		mutedConversations()
			.then((held) => (muted = held))
			.catch(() => undefined);
		const cached = loadCachedConversations();
		if (cached.length > 0) {
			conversations = cached;
			selectInitialChannel();
		}
		try {
			await loadConversationList();
			if (!conversations.some((conversation) => conversation.id === activeID)) {
				selectInitialChannel();
			}
		} catch {
			return;
		}
	});
</script>

<svelte:head>
	<title>{text.messenger}</title>
</svelte:head>


<Sheet.Root bind:open={isChannelSheetOpen}>
	<Sheet.Content side="left" class="gap-0 p-0" closeLabel={text.closeChannelList}>
		<Sheet.Header class="sr-only">
			<Sheet.Title>{text.channelListTitle}</Sheet.Title>
			<Sheet.Description>{text.channelListDescription}</Sheet.Description>
		</Sheet.Header>
		<MessengerChannelList
			{activeID}
			{directMessages}
			{groupChannels}
			{muted}
			{switchMuted}
			openNewDirectMessage={openNewDirectMessageFromSheet}
			openNewChannel={canManageChannels ? openNewChannel : undefined}
			openBrowseChannels={canManageChannels ? openBrowseChannels : undefined}
			{openOnPlatform}
			{reorderChannels}
			selectChannel={selectChannelFromSheet}
			sidebarWidth="100%"
		/>
	</Sheet.Content>

	<div class="flex min-h-0 w-full flex-1">
		<div class="flex min-h-0 w-full flex-row overflow-hidden">
			<MessengerChannelList
				class="border-r max-sm:hidden"
				{activeID}
				{directMessages}
				{groupChannels}
				{muted}
				{switchMuted}
				{openNewDirectMessage}
				openNewChannel={canManageChannels ? openNewChannel : undefined}
				openBrowseChannels={canManageChannels ? openBrowseChannels : undefined}
				{openOnPlatform}
				{reorderChannels}
				{selectChannel}
			/>
			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<header class="flex h-14 shrink-0 items-center gap-2 border-b px-6">
					<Sheet.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-sm"
								class="-ml-2 shrink-0 sm:hidden"
								aria-label={text.openChannelList}
							>
								<PanelLeftIcon />
							</Button>
						{/snippet}
					</Sheet.Trigger>
					{#if activeConversation?.kind === 'dm'}
						<PersonAvatar
							name={activeConversation.name}
							seed={activeConversation.id}
							image={activeConversation.avatarURL ?? ''}
							memberID={activeConversation.counterpart?.memberID ?? ''}
							externalID={activeConversation.counterpart?.externalID ?? ''}
							class="size-6"
						/>

					{:else if activeConversation}
						<HashIcon class="text-muted-foreground size-5 shrink-0" />
					{/if}
					<span class="truncate font-semibold">{activeConversation?.name ?? text.messenger}</span>
					{#if activeConversation?.kind === 'group' && activeConversation.members}
						<MessengerChannelHeaderActions
							memberCount={activeConversation.members.length}
							openMembers={openChannelMembers}
							openDetails={openChannelDetails}
						/>
					{/if}
				</header>
				{#key activeID}
					<Channel
						channelId={activeID}
						participants={mentionPeopleOf(activeConversation)}
						isGroup={activeConversation?.kind === 'group'}
						showSenderNames={activeConversation?.kind === 'group'}
						canModerate={activeConversation?.myRole === 'owner' || activeConversation?.myRole === 'admin'}
					/>
				{/key}
			</div>
		</div>
	</div>
</Sheet.Root>

{#if activeConversation?.kind === 'group'}
	<MessengerChannelDetails
		bind:open={isChannelDetailsOpen}
		channel={activeConversation}
		openMembers={() => ((isChannelDetailsOpen = false), openChannelMembers())}
		openOwnerAdd={() => ((isChannelDetailsOpen = false), (channelOwnerAction = 'add'), (isChannelOwnerOpen = true))}
		openOwnerHandover={() => (
			(isChannelDetailsOpen = false), (channelOwnerAction = 'hand-over'), (isChannelOwnerOpen = true)
		)}
		onLeft={leftChannel}
		onDeleted={leftChannel}
	/>
	<MessengerChannelMembersDialog
		bind:open={isChannelMembersOpen}
		channelID={activeConversation.id}
		members={activeConversation.members ?? []}
		viewerRole={activeConversation.myRole}
		onMembersChanged={() => refreshConversations('the channel list did not refresh after adding members')}
	/>
	<MessengerChannelOwnerDialog
		bind:open={isChannelOwnerOpen}
		channelID={activeConversation.id}
		members={activeConversation.members ?? []}
		action={channelOwnerAction}
		onHandedOver={() => refreshConversations('the channel list did not refresh after handing the channel over')}
	/>
{/if}
<MessengerNewChannelDialog bind:open={isNewChannelOpen} onCreated={showChannel} />
<MessengerBrowseChannelsDialog bind:open={isBrowseChannelsOpen} onJoined={showChannel} />

<Dialog.Root bind:open={isNewDirectMessageOpen}>
	<Dialog.Content class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>{text.newDirectMessage}</Dialog.Title>
		</Dialog.Header>
		<div class="-mx-2 max-h-80 overflow-y-auto">
			<Button variant="ghost" class="w-full justify-start" onclick={startAgentConversation}>{text.openLabel}</Button>
			{#each people as person (person.id)}
				<button
					type="button"
					class="hover:bg-muted/60 flex w-full items-center gap-3 rounded-md px-2 py-2 text-left"
					onclick={() => startDirectMessage(person)}
				>
					<PersonAvatar
						name={person.name}
						seed={person.id}
						memberID={person.id}
						image={person.avatarURL ?? ''}
						class="size-8"
					/>
					<span class="truncate text-sm font-medium">{displayPersonName(person.name)}</span>
				</button>
			{/each}
			{#if people.length === 0}
				<p class="text-muted-foreground px-2 py-6 text-center text-sm">{text.noPeople}</p>
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>
