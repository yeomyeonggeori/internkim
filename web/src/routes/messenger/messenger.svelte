<script lang="ts">
	import * as Empty from '$lib/components/ui/empty';
	import Channel from '$lib/components/channel/channel.svelte';
	import ChannelLoadingSkeleton from '$lib/components/channel/channel-loading-skeleton.svelte';
	import MessengerChannelList from './messenger-channel-list.svelte';
	import MessengerListSkeleton from './messenger-list-skeleton.svelte';
	import MessengerBrowseChannelsDialog from './messenger-browse-channels-dialog.svelte';
	import MessengerChannelDetails from './messenger-channel-details.svelte';
	import MessengerChannelHeaderActions from './messenger-channel-header-actions.svelte';
	import MessengerChannelMembersDialog from './messenger-channel-members-dialog.svelte';
	import MessengerChannelOwnerDialog from './messenger-channel-owner-dialog.svelte';
	import MessengerConversationExportDialog from './messenger-conversation-export-dialog.svelte';
	import MessengerLeaveChannelDialog from './messenger-leave-channel-dialog.svelte';
	import MessengerNewChannelDialog from './messenger-new-channel-dialog.svelte';
	import MessengerPersonProfile from './messenger-person-profile.svelte';
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
	import { mentionPeopleOf, type MentionPerson } from '$lib/messenger/mention-candidates';
	import { personProfileContext, type PersonProfileActions } from '$lib/messenger/person-profile';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		ensureDirectMessage,
		ensureAgentConversation,
		fetchConversations,
		fetchPeople,
		type ChannelSummary,
		type Person
	} from '$lib/components/channel/channel-api';
	import { onDestroy, onMount, setContext } from 'svelte';
	import { onCompanyEvent } from '$lib/host-bridge';
	import type { CompanyEvent } from '$lib/company-event';
	import { isSupabaseConfigured } from '$lib/supabase';
	import { keepCentralBuzzIdentity } from '$lib/central-buzz-identity';
	import { markChannelRead } from '$lib/messenger/messenger-api';
	import { createConversationReadMarker } from '$lib/messenger/conversation-read-marker';
	import { page } from '$app/state';
	import { afterNavigate, replaceState } from '$app/navigation';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { syncMattermostToBuzz } from '$lib/buzz-mm-sync';
	import { loadChannelOrder, saveChannelOrder, orderChannels, moveChannel } from './channel-order';
	import { holdBackNotificationsFor } from '$lib/native-shell/open-conversation';
	import { adoptRememberedMessengerScope, messengerCacheScope, messengerCacheKey, onMessengerCacheReset, requireCurrentMessengerScope, type MessengerCacheScope } from '$lib/messenger/cache-scope';
	import { keepCachedConversations, readCachedConversations } from '$lib/messenger/conversation-list-cache';

	const lastChannelKey = 'messenger-last-channel';

	function loadLastChannelID(): string | null {
		if (typeof localStorage === 'undefined') return null;
		const scope = messengerCacheKey();
		return scope ? localStorage.getItem(`${lastChannelKey}:${scope}`) : null;
	}

	function selectChannel(channelID: string) {
		activeID = channelID;
		const scope = messengerCacheKey();
		if (typeof localStorage !== 'undefined' && scope) localStorage.setItem(`${lastChannelKey}:${scope}`, channelID);
		const url = new URL(location.href);
		url.searchParams.set('channel', channelID);
		replaceState(url, {});
	}

	const text = createPageText(channelText);

	let conversations = $state<ChannelSummary[]>([]);
	let isLoadingConversations = $state(true);
	let hasLoadedConversations = $state(false);
	let conversationError = $state('');
	let conversationReadSequence = 0;
	let isDestroyed = false;
	let activeID = $state<string | undefined>(undefined);
	let cacheGeneration = $state(0);
	let isNewDirectMessageOpen = $state(false);
	let isNewChannelOpen = $state(false);
	let isBrowseChannelsOpen = $state(false);
	let isChannelDetailsOpen = $state(false);
	let isExportOpen = $state(false);
	let exportingConversation = $state<ChannelSummary | null>(null);

	function exportConversation(conversation: ChannelSummary) {
		exportingConversation = conversation;
		isExportOpen = true;
	}
	let isLeaveOpen = $state(false);
	let leavingChannel = $state<ChannelSummary | null>(null);

	function leaveChannel(channel: ChannelSummary) {
		leavingChannel = channel;
		isLeaveOpen = true;
	}
	let isChannelMembersOpen = $state(false);
	let isChannelOwnerOpen = $state(false);
	let channelOwnerAction = $state<'add' | 'hand-over'>('add');
	const canManageChannels = isSupabaseConfigured();
	let isChannelSheetOpen = $state(false);
	let people = $state<Person[]>([]);
	let isLoadingPeople = $state(false);
	let peopleError = $state('');
	let peopleReadSequence = 0;
	let muted = $state<Set<string>>(new Set());
	let syncedIdentityRevision = -1;
	let userChannelOrder = $state<string[]>([]);
	let profilePerson = $state<MentionPerson | undefined>(undefined);
	let isPersonProfileOpen = $state(false);

	function messagePerson(person: MentionPerson) {
		isPersonProfileOpen = false;
		void startDirectMessage({ id: person.externalID, name: person.name });
	}

	setContext<PersonProfileActions>(personProfileContext, {
		show: (person) => ((profilePerson = person), (isPersonProfileOpen = true)),
		message: messagePerson
	});

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
		if (activeID !== channelID) {
			refreshConversations('the channel list did not refresh after leaving');
			return;
		}
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

	async function loadConversationList() {
		if (isDestroyed) return;
		const sequence = ++conversationReadSequence;
		isLoadingConversations = true;
		conversationError = '';
		try {
			const scope = await messengerCacheScope();
			const next = await fetchConversations();
			await requireCurrentMessengerScope(scope);
			if (sequence !== conversationReadSequence) return;
			conversations = next;
			hasLoadedConversations = true;
			if (activeID === undefined) selectInitialChannel();
			keepCachedConversations(scope, conversations);
		} catch (failure) {
			if (sequence === conversationReadSequence) conversationError = failure instanceof Error ? failure.message : text.unavailableDescription;
			throw failure;
		} finally {
			if (sequence === conversationReadSequence) isLoadingConversations = false;
		}
	}

	function selectInitialChannel() {
		if (isDestroyed) return;
		const requestedID = page.url.searchParams.get('channel') ?? loadLastChannelID();
		const remembered = requestedID
			? conversations.find((conversation) => conversation.id === requestedID)
			: undefined;
		const initial = remembered ?? groupChannels[0] ?? conversations[0];
		if (initial) selectChannel(initial.id);
	}

	afterNavigate((navigation) => {
		const requestedID = navigation.to?.url.searchParams.get('channel');
		if (!requestedID || activeID === undefined || requestedID === activeID) return;
		void showChannel(requestedID);
	});

	$effect(() => {
		const revision = buzzIdentity.revision;
		if (!buzzIdentity.secretHex || syncedIdentityRevision === revision) return;
		syncedIdentityRevision = revision;
		syncMattermostToBuzz(buzzIdentity.secretHex, () => buzzIdentity.revision === revision)
			.then(() => { if (buzzIdentity.revision === revision) return loadConversationList(); })
			.catch(() => {});
	});

	async function openNewDirectMessage() {
		isNewDirectMessageOpen = true;
		const sequence = ++peopleReadSequence;
		isLoadingPeople = true;
		peopleError = '';
		try {
			const scope = await messengerCacheScope();
			const next = await fetchPeople();
			await requireCurrentMessengerScope(scope);
			if (sequence !== peopleReadSequence) return;
			people = next;
		} catch (failure) {
			if (sequence === peopleReadSequence) {
				people = [];
				peopleError = failure instanceof Error ? failure.message : text.unavailableDescription;
			}
		} finally {
			if (sequence === peopleReadSequence) isLoadingPeople = false;
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
	let stopFollowingCacheScope = () => {};

	const markReadThrough = createConversationReadMarker(markChannelRead);

	function readThrough(conversationID: string | undefined, sentAt: string): void {
		if (!conversationID || !isSupabaseConfigured()) return;
		markReadThrough(conversationID, sentAt)
			.then((isNewlyRead) => {
				if (!isNewlyRead) return;
				conversations = conversations.map((conversation) =>
					conversation.id === conversationID ? { ...conversation, unreadCount: 0 } : conversation
				);
			})
			.catch((failure: unknown) => console.warn('the messenger did not record how far this conversation was read', failure));
	}

	$effect(() => {
		holdBackNotificationsFor(activeID).catch((failure: unknown) =>
			console.warn('the shell is not holding back this conversation', failure)
		);
	});

	onDestroy(() => {
		isDestroyed = true;
		conversationReadSequence += 1;
		peopleReadSequence += 1;
		stopListeningForArrivals();
		stopFollowingCacheScope();
		holdBackNotificationsFor(undefined).catch((failure: unknown) =>
			console.warn('the shell is still holding back a closed conversation', failure)
		);
	});

	onMount(keepCentralBuzzIdentity);
	onMount(async () => {
		stopFollowingCacheScope = onMessengerCacheReset(() => {
			cacheGeneration += 1;
			conversationReadSequence += 1;
			peopleReadSequence += 1;
			hasLoadedConversations = false;
			conversationError = '';
			isLoadingPeople = false;
			peopleError = '';
			conversations = [];
			people = [];
			activeID = undefined;
			muted = new Set();
			profilePerson = undefined;
			isPersonProfileOpen = false;
			isNewDirectMessageOpen = false;
			isBrowseChannelsOpen = false;
			isNewChannelOpen = false;
			conversationsNotMine.clear();
			void loadConversationList().then(selectInitialChannel).catch(() => undefined);
		});
		userChannelOrder = loadChannelOrder();
		if (isSupabaseConfigured()) stopListeningForArrivals = onCompanyEvent(readListOnArrival);

		const mountedGeneration = cacheGeneration;
		const remembered = await adoptRememberedMessengerScope();
		if (isDestroyed || mountedGeneration !== cacheGeneration) return;
		const cached = remembered ? readCachedConversations(remembered) : [];
		if (cached.length > 0) {
			conversations = cached;
			hasLoadedConversations = true;
			selectInitialChannel();
		}
		let scope: MessengerCacheScope;
		try { scope = await messengerCacheScope(); } catch (failure) {
			if (isDestroyed || mountedGeneration !== cacheGeneration) return;
			conversationError = failure instanceof Error ? failure.message : text.unavailableDescription;
			isLoadingConversations = false;
			return;
		}
		if (isDestroyed || mountedGeneration !== cacheGeneration) return;
		mutedConversations()
			.then(async (held) => { await requireCurrentMessengerScope(scope); muted = held; })
			.catch(() => undefined);
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
		<Sheet.Header class="min-h-16 justify-center border-b px-4 py-3">
			<Sheet.Title>{text.channelListTitle}</Sheet.Title>
			<Sheet.Description class="sr-only">{text.channelListDescription}</Sheet.Description>
		</Sheet.Header>
		<MessengerChannelList
			isLoading={isLoadingConversations && !hasLoadedConversations}
			error={conversationError}
			retry={() => refreshConversations('the channel list did not refresh')}
			{activeID}
			{directMessages}
			{groupChannels}
			{muted}
			{switchMuted}
			{exportConversation}
			{leaveChannel}
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
				isLoading={isLoadingConversations && !hasLoadedConversations}
				error={conversationError}
				retry={() => refreshConversations('the channel list did not refresh')}
				{activeID}
				{directMessages}
				{groupChannels}
				{muted}
				{switchMuted}
				{exportConversation}
				{leaveChannel}
				{openNewDirectMessage}
				openNewChannel={canManageChannels ? openNewChannel : undefined}
				openBrowseChannels={canManageChannels ? openBrowseChannels : undefined}
				{openOnPlatform}
				{reorderChannels}
				{selectChannel}
			/>
			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<header class="flex h-12 shrink-0 items-center gap-2 border-b px-2 sm:h-14 sm:px-6">
					<Sheet.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-sm"
								class="shrink-0 sm:hidden"
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
				{#if !hasLoadedConversations && isLoadingConversations}
					<ChannelLoadingSkeleton label={text.loadingConversations} />
				{:else if !hasLoadedConversations && conversationError}
					<div class="grid flex-1 place-content-center gap-3 p-6"><p role="alert" class="text-sm text-destructive">{conversationError}</p><Button variant="outline" onclick={() => refreshConversations('the channel list did not refresh')}>{text.retry}</Button></div>
				{:else}
				{#key `${cacheGeneration}:${activeID}`}
					<Channel
						channelId={activeID}
						participants={mentionPeopleOf(activeConversation)}
						isGroup={activeConversation?.kind === 'group'}
						showSenderNames={activeConversation?.kind === 'group'}
						canModerate={activeConversation?.myRole === 'owner' || activeConversation?.myRole === 'admin'}
						isWithTheAgent={activeConversation?.isWithTheAgent === true}
						onReadThrough={(sentAt) => readThrough(activeID, sentAt)}
					/>
				{/key}
				{/if}
			</div>
		</div>
	</div>
</Sheet.Root>

<MessengerConversationExportDialog bind:open={isExportOpen} conversation={exportingConversation} />

<MessengerLeaveChannelDialog bind:open={isLeaveOpen} channel={leavingChannel} onLeft={leftChannel} />

<MessengerPersonProfile bind:open={isPersonProfileOpen} person={profilePerson} onMessage={messagePerson} />

{#if activeConversation?.kind === 'group'}
	<MessengerChannelDetails
		bind:open={isChannelDetailsOpen}
		channel={activeConversation}
		openMembers={() => ((isChannelDetailsOpen = false), openChannelMembers())}
		openOwnerAdd={() => ((isChannelDetailsOpen = false), (channelOwnerAction = 'add'), (isChannelOwnerOpen = true))}
		openOwnerHandover={() => (
			(isChannelDetailsOpen = false), (channelOwnerAction = 'hand-over'), (isChannelOwnerOpen = true)
		)}
		openExport={() => ((isChannelDetailsOpen = false), exportConversation(activeConversation))}
		openLeave={() => ((isChannelDetailsOpen = false), leaveChannel(activeConversation))}
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
				{#if peopleError}
					<div class="space-y-2 px-2 py-3"><p role="alert" class="text-destructive text-sm">{peopleError}</p><Button variant="outline" size="sm" onclick={openNewDirectMessage}>{text.retry}</Button></div>
				{/if}
				{#if isLoadingPeople && !people.length}
					<MessengerListSkeleton label={text.loadingPeople} />
				{:else}
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
				{#if people.length === 0 && !peopleError}
					<Empty.Root><Empty.Header><Empty.Title>{text.noPeople}</Empty.Title></Empty.Header></Empty.Root>
				{/if}
				{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>
