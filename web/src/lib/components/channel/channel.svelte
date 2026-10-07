<script lang="ts">
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Marker from '$lib/components/ui/marker/index.js';
	import * as Message from '$lib/components/ui/message/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ConversationDock from './conversation-dock.svelte';
	import { createChannelTyping } from './channel-typing.svelte';
	import { agentWorkingRefreshIntervalMs, stillWorkingSince } from './agent-working';
	import ChannelMessageBody from './channel-message-body.svelte';
	import ChannelComposer, { type OutgoingMessage } from './channel-composer.svelte';
	import ChannelLightbox, { type LightboxView } from './channel-lightbox.svelte';
	import ChannelLinkPreview from './channel-link-preview.svelte';
	import FailedMessageActions from './failed-message-actions.svelte';
	import { createOutgoingMessages } from './outgoing-messages.svelte';
	import MessageReactions from './message-reactions.svelte';
	import MessageRow from './message-row.svelte';
	import MessageCaptureBar from './message-capture-bar.svelte';
	import MessageCaptureOverlay from './message-capture-overlay.svelte';
	import { createMessageCapture } from './message-capture.svelte';
	import { chooseMessageOnClick } from './choose-message-on-click';
	import { messageTextBeside } from './message-text-beside';
	import { canEditMessage, type EditingMessage } from './message-edit';
	import { firstLinkIn } from './channel-link';
	import { clockTime, dateKeyOf, dateLabel, relativeTime } from './channel-time';
	import { threadRepliesByRoot, timelineMessages } from './channel-threads';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import LoadingImage from '$lib/components/loading-image.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		canChangeMessages,
		fetchChannelConversation,
		applyCustomEmoji,
		sendChannelMessage,
		type ChannelMessage,
		type ChannelParticipant,
		type ThreadSummary
	} from './channel-api';
	import {
		attachmentStateOf,
		formatAttachmentMeta,
		preparingLabel,
		openableAttachments,
		pictureAddressesOf,
		messagePicturesOf
	} from './channel-attachments';
	import { startDownload } from './attachment-download';
	import { messageActionsFor } from './channel-message-actions';
	import type { MentionPerson } from '$lib/messenger/mention-candidates';
	import { latestSentAtOf } from '$lib/messenger/conversation-read-marker';
	import { mentionLabelsOf } from '$lib/messenger/mention-picker.svelte';
	import { activityLabel } from '$lib/messenger/typing-signal';
	import { whatToCopy } from './message-copy';
	import { messagesWithReactions } from './channel-reactions';
	import {
		channelMessageCacheGeneration,
		getCachedMessages,
		getCachedReaderID,
		setCachedMessages,
		setCachedReaderID
	} from './channel-message-cache';
	import { groupConsecutiveMessages } from './channel-message-groups';
	import { jumpToLatestLabel, unseenMessageCount } from './channel-unseen-messages';
	import { leaveOpenedThreadThroughHistory, rememberOpenedThread, threadRootIDInHistory } from './thread-history';
	import CornerDownRightIcon from '@lucide/svelte/icons/corner-down-right';
	import FileIcon from '@lucide/svelte/icons/file';
	import InfoIcon from '@lucide/svelte/icons/info';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import MessageCircleDashedIcon from '@lucide/svelte/icons/message-circle-dashed';
	import XIcon from '@lucide/svelte/icons/x';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ChannelLoadingSkeleton from './channel-loading-skeleton.svelte';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { customEmoji } from '$lib/stores/custom-emoji.svelte';
	import { isCurrentMessengerScope, messengerCacheScope, onMessengerCacheReset, requireCurrentMessengerScope } from '$lib/messenger/cache-scope';
	import { attachmentSource } from '$lib/stores/attachment-source.svelte';
	import { onCompanyEvent } from '$lib/host-bridge';
	import type { CompanyEvent } from '$lib/company-event';
	import { isSupabaseConfigured } from '$lib/supabase';
	import { onDestroy, onMount, untrack } from 'svelte';

	let {
		isActive = true,
		threadLayout = 'sheet',
		channelId,
		showSenderNames = false,
		canModerate = false,
		participants = [],
		isGroup = false,
		isWithTheAgent = true,
		onReadThrough
	}: {
		isActive?: boolean;
		threadLayout?: 'sheet' | 'inline';
		channelId?: string;
		showSenderNames?: boolean;
		canModerate?: boolean;
		participants?: MentionPerson[];
		isGroup?: boolean;
		isWithTheAgent?: boolean;
		onReadThrough?: (sentAt: string) => void;
	} = $props();

	const text = createPageText(channelText);
	const idleRefreshIntervalMs = isSupabaseConfigured() ? 30_000 : 5000;

	let messages = $state<ChannelMessage[]>([]);
	let currentUserID = $state(getCachedReaderID());
	let currentUserEmail = $state('');
	let currentUserImage = $state('');
	let agentWorkingSince = $state<number | null>(null);
	const isAgentWorking = $derived(agentWorkingSince !== null);
	const nameByExternalID = $derived(new Map(participants.map((person) => [person.externalID, person.name])));
	const typing = createChannelTyping(
		() => channelId,
		() => messages.map((message) => ({ senderExternalID: message.sender.externalID, sentAt: message.sentAt }))
	);
	const activity = $derived(
		activityLabel(
			{
				typerExternalIDs: typing.typerExternalIDs,
				nameOf: (externalID) => nameByExternalID.get(externalID),
				isGroup,
				isAgentWorking
			},
			text
		)
	);
	let isSending = $state(false);
	let dockHeight = $state(0);
	let threadEditing = $state<EditingMessage | null>(null);
	let conversationComposer = $state<ChannelComposer | null>(null);
	let threadComposer = $state<ChannelComposer | null>(null);
	let loadFailed = $state(false);
	let hasLoadedOnce = $state(false);
	let olderMessages = $state<ChannelMessage[]>([]);
	let hasMoreBefore = $state(false);
	let historyCursor = $state('');
	let isLoadingOlder = $state(false);
	let lightbox = $state<LightboxView | null>(null);
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;
	let isDestroyed = false;
	let channelGeneration = 0;
	let nextReadSequence = 0;

	function isCurrentChannel(
		requestedChannelID: string | undefined,
		generation: number,
		cacheGeneration: number
	): boolean {
		return !isDestroyed && channelId === requestedChannelID && channelGeneration === generation &&
			channelMessageCacheGeneration() === cacheGeneration;
	}

	function isCurrentRead(
		requestedChannelID: string | undefined,
		generation: number,
		cacheGeneration: number,
		readSequence: number
	): boolean {
		return readSequence === nextReadSequence && isCurrentChannel(requestedChannelID, generation, cacheGeneration);
	}

	function openLightbox(attachments: NonNullable<ChannelMessage['attachments']>, source: string) {
		const images = attachments
			.filter(attachment => attachment.kind === 'image' && attachment.source)
			.map(attachment => ({ source: attachment.source ?? '', filename: attachment.filename ?? '', width: attachment.widthPixels, height: attachment.heightPixels }));
		if (images.length === 0) return;
		lightbox = { images, index: Math.max(0, images.findIndex(image => image.source === source)) };
	}
	let lastConversationSignature = '';
	let cacheGeneration = $state(0);
	let openThreadRoot = $state<ChannelMessage | null>(null);
	let isThreadSending = $state(false);

	const currentUser = $derived<ChannelParticipant>({
		id: currentUserID,
		name: text.you,
		email: currentUserEmail,
		avatarURL: currentUserImage
	});

	function isMine(message: ChannelMessage): boolean {
		return currentUserID !== '' && message.sender.id === currentUserID;
	}

	const canChange = canChangeMessages();
	const openableAddressOf = (url: string): string => attachmentSource.openable(url);
	const messageActions = messageActionsFor({
		channelID: () => channelId,
		reader: () => currentUser,
		text: () => text,
		showReactions: (messageID, reactions) => {
			messages = messagesWithReactions(messages, messageID, reactions);
			olderMessages = messagesWithReactions(olderMessages, messageID, reactions);
			if (openThreadRoot?.id === messageID) openThreadRoot = { ...openThreadRoot, reactions };
		},
		forgetMessage: (messageID) => {
			messages = messages.filter((message) => message.id !== messageID);
			olderMessages = olderMessages.filter((message) => message.id !== messageID);
			if (openThreadRoot?.id === messageID) closeThread();
		},
		readAgain: loadConversation
	});

	const outgoing = createOutgoingMessages({
		sender: () => currentUser,
		deliver: (entry) =>
			sendChannelMessage(
				entry.draft.text,
				entry.draft.attachments,
				entry.channelID,
				entry.threadRootID,
				entry.draft.mentions
			),
		afterDelivered: async (entry) => {
			if (!entry.threadRootID) agentWorkingSince = isWithTheAgent ? Date.now() : null;
			await loadConversation();
		}
	});

	const shownMessages = $derived([...messages, ...outgoing.messagesIn(channelId)]);

	const repliesByRoot = $derived(threadRepliesByRoot(shownMessages));

	const visibleMessages = $derived(timelineMessages(shownMessages, repliesByRoot));

	const messageGroups = $derived(groupConsecutiveMessages(visibleMessages));

	const capture = createMessageCapture(() => visibleMessages.map((message) => message.id));

	$effect(() => {
		void channelId;
		untrack(() => capture.end());
	});

	type TimelineItem =
		| { kind: 'date'; id: string; label: string }
		| { kind: 'group'; id: string; senderID: string; items: ChannelMessage[] };

	const timeline = $derived.by(() => {
		const items: TimelineItem[] = [];
		let previousDateKey = '';
		for (const group of messageGroups) {
			const dateKey = dateKeyOf(group.items[0].sentAt);
			if (dateKey !== previousDateKey) {
				items.push({ kind: 'date', id: `date-${dateKey}`, label: dateLabel(group.items[0].sentAt) });
				previousDateKey = dateKey;
			}
			items.push({ kind: 'group', ...group });
		}
		return items;
	});

	const threadReplies = $derived(
		openThreadRoot ? (repliesByRoot.get(openThreadRoot.id) ?? []) : []
	);

	const threadReplyGroups = $derived(groupConsecutiveMessages(threadReplies));

	async function loadConversation() {
		if (isDestroyed) return;
		const requestedChannelID = channelId;
		const generation = channelGeneration;
		let messageCacheGeneration = channelMessageCacheGeneration();
		const readSequence = ++nextReadSequence;
		const deliveredBeforeReading = outgoing.deliveredSoFar();
		let scope: Awaited<ReturnType<typeof messengerCacheScope>> | undefined;
		try {
			scope = await messengerCacheScope();
			messageCacheGeneration = channelMessageCacheGeneration();
			if (!isCurrentRead(requestedChannelID, generation, messageCacheGeneration, readSequence)) return;
			const conversation = await fetchChannelConversation(requestedChannelID);
			await requireCurrentMessengerScope(scope);
			if (!isCurrentRead(requestedChannelID, generation, messageCacheGeneration, readSequence)) return;
			currentUserID = conversation.currentUserID;
			setCachedReaderID(conversation.currentUserID);
			const latestIncoming = conversation.messages.at(-1);
			if (latestIncoming && !isMine(latestIncoming) && latestIncoming.id !== messages.at(-1)?.id) {
				agentWorkingSince = null;
			}
			// Only replace the list when it actually changed. A poll that returns the
			// same messages must not reassign the array, or the re-render resets the
			// scroll position and the view keeps jumping.
			if (olderMessages.length === 0) {
				hasMoreBefore = conversation.hasMoreBefore;
				historyCursor = conversation.historyCursor;
			}
			const signature = conversationSignature(conversation.messages);
			if (signature !== lastConversationSignature) {
				lastConversationSignature = signature;
				messages = mergeOlderMessages(olderMessages, conversation.messages);
				setCachedMessages(requestedChannelID, conversation.messages);
				if (openThreadRoot) {
					openThreadRoot = visibleMessages.find((message) => message.id === openThreadRoot?.id) ?? openThreadRoot;
				}
			}
			outgoing.forgetDeliveredThrough(deliveredBeforeReading);
			loadFailed = false;
		} catch {
			if ((!scope || isCurrentMessengerScope(scope)) &&
				isCurrentRead(requestedChannelID, generation, messageCacheGeneration, readSequence)) loadFailed = true;
		} finally {
			if ((!scope || isCurrentMessengerScope(scope)) &&
				isCurrentRead(requestedChannelID, generation, messageCacheGeneration, readSequence)) hasLoadedOnce = true;
		}
	}

	function mergeOlderMessages(older: ChannelMessage[], latest: ChannelMessage[]): ChannelMessage[] {
		const latestIds = new Set(latest.map((message) => message.id));
		const head = older.filter((message) => !latestIds.has(message.id));
		return [...head, ...latest];
	}

	// The message list is a column-reverse scroller: newest at the bottom, scroll
	// origin at the bottom. Prepending older messages appends them to the far
	// (top) end, so the browser keeps the reading position natively — no scrollTop
	// math and nothing for an autoscroll to fight.
	let scrollContainer = $state<HTMLDivElement | null>(null);
	let showScrollToBottom = $state(false);
	const reversedTimeline = $derived(timeline.slice().reverse());
	let isPageVisible = $state(typeof document === 'undefined' || document.visibilityState === 'visible');
	const latestSettledSentAt = $derived(latestSentAtOf(messages));
	let seenThroughSentAt = $state('');
	const unseenCount = $derived(showScrollToBottom ? unseenMessageCount(messages, seenThroughSentAt, isMine) : 0);

	$effect(() => {
		if (!showScrollToBottom) seenThroughSentAt = latestSettledSentAt;
	});

	$effect(() => {
		if (!onReadThrough || !isPageVisible || showScrollToBottom || !latestSettledSentAt) return;
		onReadThrough(latestSettledSentAt);
	});

	const olderPrefetchScreens = 3;

	function distanceFromOldestTop(container: HTMLElement): number {
		return container.scrollHeight - container.clientHeight - Math.abs(container.scrollTop);
	}

	async function loadOlderMessages() {
		if (isLoadingOlder || !hasMoreBefore || !historyCursor) return;
		const requestedChannelID = channelId;
		const generation = channelGeneration;
		let messageCacheGeneration = channelMessageCacheGeneration();
		const requestedCursor = historyCursor;
		isLoadingOlder = true;
		try {
			const scope = await messengerCacheScope();
			messageCacheGeneration = channelMessageCacheGeneration();
			if (!isCurrentChannel(requestedChannelID, generation, messageCacheGeneration) || historyCursor !== requestedCursor)
				return;
			const page = await fetchChannelConversation(requestedChannelID, requestedCursor);
			await requireCurrentMessengerScope(scope);
			if (!isCurrentChannel(requestedChannelID, generation, messageCacheGeneration) || historyCursor !== requestedCursor)
				return;
			hasMoreBefore = page.hasMoreBefore;
			historyCursor = page.historyCursor;
			const existingIds = new Set(messages.map((message) => message.id));
			const fresh = page.messages.filter((message) => !existingIds.has(message.id));
			if (fresh.length === 0) {
				hasMoreBefore = false;
				return;
			}
			olderMessages = [...fresh, ...olderMessages];
			messages = [...fresh, ...messages];
		} finally {
			if (isCurrentChannel(requestedChannelID, generation, messageCacheGeneration)) isLoadingOlder = false;
		}
		if (!isCurrentChannel(requestedChannelID, generation, messageCacheGeneration)) return;
		// Only keep a small buffer ahead of the fold as the user scrolls up — load
		// on demand, never speculatively, so we don't pay for history nobody reads.
		requestAnimationFrame(prefetchOlderIfNearTop);
	}

	function prefetchOlderIfNearTop() {
		const container = scrollContainer;
		if (!container || !hasMoreBefore) return;
		if (distanceFromOldestTop(container) < container.clientHeight * olderPrefetchScreens) {
			loadOlderMessages();
		}
	}

	function handleViewportScroll() {
		const container = scrollContainer;
		if (!container) return;
		showScrollToBottom = Math.abs(container.scrollTop) > 200;
		prefetchOlderIfNearTop();
	}

	function scrollToBottom() {
		scrollContainer?.scrollTo({ top: 0, behavior: 'smooth' });
	}

	function conversationSignature(list: ChannelMessage[]): string {
		return list
			.map((message) => {
				const reactionTotal = (message.reactions ?? []).reduce((sum, reaction) => sum + reaction.count, 0);
				return `${message.id}:${message.sentAt}:${message.text.length}:${reactionTotal}:${(message.attachments ?? []).length}`;
			})
			.join('~');
	}

	async function loadCurrentUser() {
		const cacheGeneration = channelMessageCacheGeneration();
		try {
			const response = await fetch('/auth/session', { credentials: 'include', cache: 'no-store' });
			if (!response.ok) return;
			const session: { authenticated?: boolean; email?: string; image?: string } = await response.json();
			if (!isDestroyed && cacheGeneration === channelMessageCacheGeneration() && session.authenticated) {
				currentUserEmail = session.email ?? '';
				currentUserImage = session.image ?? '';
			}
		} catch {
			if (!isDestroyed && cacheGeneration === channelMessageCacheGeneration()) {
				currentUserEmail = '';
				currentUserImage = '';
			}
		}
	}

	function openThread(rootMessage: ChannelMessage) {
		rememberOpenedThread(rootMessage.id);
		openThreadRoot = rootMessage;
	}

	async function sendThreadReply(draft: OutgoingMessage) {
		if (!openThreadRoot) return;
		await outgoing.send(draft, channelId, openThreadRoot.id);
	}

	function scheduleRefresh() {
		clearTimeout(refreshTimer);
		if (isDestroyed) return;
		agentWorkingSince = stillWorkingSince(agentWorkingSince, Date.now());
		if (!isActive) return;
		const delay = isAgentWorking ? agentWorkingRefreshIntervalMs : idleRefreshIntervalMs;
		refreshTimer = setTimeout(async () => {
			await loadConversation();
			scheduleRefresh();
		}, delay);
	}

	let isReadingAgain = false;
	let isAnotherReadWanted = false;

	async function readAgainOnArrival(event: CompanyEvent) {
		if (event.kind !== 'message.arrived' || !channelId || event.conversationID !== channelId) return;
		if (isReadingAgain) {
			isAnotherReadWanted = true;
			return;
		}
		isReadingAgain = true;
		try {
			do {
				isAnotherReadWanted = false;
				await loadConversation();
			} while (isAnotherReadWanted);
		} finally {
			isReadingAgain = false;
		}
		scheduleRefresh();
	}

	async function sendToConversation(draft: OutgoingMessage) {
		const sending = outgoing.send(draft, channelId);
		scrollContainer?.scrollTo({ top: 0 });
		await sending;
		scheduleRefresh();
	}

	async function sendAgain(messageID: string) {
		await outgoing.retry(messageID);
		scheduleRefresh();
	}

	function closeThread() {
		if (leaveOpenedThreadThroughHistory()) return;
		dismissThread();
	}

	function dismissThread() {
		threadComposer?.clear();
		openThreadRoot = null;
	}

	$effect(() => {
		if (threadRootIDInHistory() === undefined && untrack(() => openThreadRoot) !== null) dismissThread();
	});

	async function answerChoice(optionLabel: string) {
		if (isSending) return;
		isSending = true;
		try {
			await sendChannelMessage(optionLabel, [], channelId);
			agentWorkingSince = isWithTheAgent ? Date.now() : null;
			await loadConversation();
		} catch {
			loadFailed = true;
		} finally {
			isSending = false;
			scheduleRefresh();
		}
	}

	$effect(() => {
		if (isActive) scheduleRefresh();
		else clearTimeout(refreshTimer);
	});

	$effect(() => {
		const activeChannelID = channelId;
		channelGeneration += 1;
		olderMessages = [];
		hasMoreBefore = false;
		historyCursor = '';
		isLoadingOlder = false;
		openThreadRoot = null;
		const cached = getCachedMessages(activeChannelID) ?? [];
		if (cached.length > 0) {
			messages = cached;
			lastConversationSignature = conversationSignature(cached);
			hasLoadedOnce = true;
		} else {
			messages = [];
			lastConversationSignature = '';
			hasLoadedOnce = false;
		}
		loadConversation();
	});

	let stopListeningForArrivals = () => {};
	let stopFollowingCacheScope = () => {};

	onMount(async () => {
		stopFollowingCacheScope = onMessengerCacheReset(() => {
			cacheGeneration += 1;
			messages = [];
			olderMessages = [];
			currentUserID = '';
			currentUserEmail = '';
			currentUserImage = '';
			hasLoadedOnce = false;
			openThreadRoot = null;
			threadEditing = null;
			lightbox = null;
			agentWorkingSince = null;
			isLoadingOlder = false;
			hasMoreBefore = false;
			historyCursor = '';
			lastConversationSignature = '';
			for (const message of outgoing.messagesIn(channelId)) outgoing.discard(message.id);
		});
		customEmoji.load();
		if (isSupabaseConfigured()) stopListeningForArrivals = onCompanyEvent((event) => void readAgainOnArrival(event));
		await loadCurrentUser();
		scheduleRefresh();
	});

	onDestroy(() => {
		isDestroyed = true;
		channelGeneration += 1;
		stopFollowingCacheScope();
		stopListeningForArrivals();
		clearTimeout(refreshTimer);
	});
</script>

{#snippet ownReactions(message: ChannelMessage, farCorner: 'start' | 'end', nameWidthPixels: number)}
	<MessageReactions
		reactions={message.reactions ?? []}
		canChange={canChange && !message.id.startsWith('pending-')}
		side="top"
		{farCorner}
		{nameWidthPixels}
		onToggle={(reaction) => messageActions.toggleReaction(message, reaction)}
	/>
{/snippet}

{#snippet messageBody(message: ChannelMessage, nameWidthPixels: number)}
	{@const attachments = openableAttachments(message.attachments ?? [], openableAddressOf)}
	{@const bodyText = messageTextBeside(
		message.text,
		attachments.map((attachment) => attachment.url)
	)}
	{@const reactions = message.reactions ?? []}
	{@const mine = isMine(message)}
	{@const reactionAlign = mine ? 'start' : 'end'}
	{@const reactionSpacing = reactions.length > 0 && nameWidthPixels === 0 ? 'mt-5' : ''}
	{@const imageReactionSpacing = bodyText ? '' : reactionSpacing}
	{@const loneImage =
		attachments.length === 1 && attachments[0].kind === 'image' && attachments[0].source
			? attachments[0]
			: undefined}
	{#if loneImage}
		<!-- One photograph is a photograph, not a thumbnail in a grid: it keeps its
		     own proportions, bounded so a tall one cannot take the whole screen.
		     Attachment.Media squares whatever it holds, which is right for a row of
		     files and wrong for this.
		     A wide picture runs out of width first and a tall one runs out of height,
		     so both bounds are here. The height is read from the window as well as
		     fixed, because 26rem of a laptop is a picture and 26rem of a short
		     window is the whole conversation.
		     The proportions come from the message when the messenger reported them,
		     so the browser holds the space before the file arrives and nothing below
		     jumps when it does. -->
		<div class={`relative w-fit max-w-[80%] self-start group-data-[align=end]/message:self-end ${imageReactionSpacing}`}>
			<button
				type="button"
				class="block max-w-full cursor-zoom-in overflow-hidden rounded-lg"
				aria-label={loneImage.filename ?? '이미지 크게 보기'}
				onclick={() => openLightbox(attachments, loneImage.source ?? '')}
			>
				<LoadingImage
					messagePicture
					src={loneImage.source ?? ''}
					alt={loneImage.filename ?? ''}
					width={loneImage.widthPixels}
					height={loneImage.heightPixels}
					loading="lazy"
					class="rounded-lg"
				/>
			</button>
			{#if !bodyText && reactions.length > 0}
				{@render ownReactions(message, reactionAlign, nameWidthPixels)}
			{/if}
			{#if !bodyText}{@render timeStamp(message)}{/if}
		</div>
	{:else if attachments.length > 0}
		<div class={`relative w-fit max-w-[80%] self-start group-data-[align=end]/message:self-end ${imageReactionSpacing}`}>
			<Attachment.Group class="relative w-fit max-w-full">
			{#each attachments as attachment (attachment.url)}
				{@const attachmentState = attachmentStateOf(attachment.source, attachmentSource.status(attachment.url))}
				<Attachment.Root orientation="vertical" state={attachmentState}>
					{#if attachment.kind === 'image' && attachment.source}
						<Attachment.Media variant="image">
							<button
								type="button"
								class="block h-full w-full cursor-zoom-in"
								aria-label={attachment.filename ?? '이미지 크게 보기'}
								onclick={() => openLightbox(attachments, attachment.source ?? '')}
							>
								<LoadingImage
									messagePicture
									src={attachment.source}
									alt={attachment.filename ?? ''}
									loading="lazy"
									fill
									class="aspect-square h-full w-full"
									imageClass="object-cover"
								/>
							</button>
						</Attachment.Media>
					{:else}
						<Attachment.Media>
							<FileIcon />
						</Attachment.Media>
						<Attachment.Content>
							<Attachment.Title>
								{#if attachment.source}
									<a href={attachment.source} target="_blank" rel="noreferrer">
										{attachment.filename ?? attachment.url}
									</a>
								{:else}
									{attachment.filename ?? attachment.url}
								{/if}
							</Attachment.Title>
							{#if attachmentState === 'error'}
								<Attachment.Description title={attachmentSource.failure(attachment.url)}>
									{text.attachmentUnavailable}
								</Attachment.Description>
							{:else if attachmentState === 'processing'}
								<Attachment.Description>
									{preparingLabel(text.attachmentPreparing, attachmentSource.progress(attachment.url))}
								</Attachment.Description>
							{:else if formatAttachmentMeta(attachment)}
								<Attachment.Description>
									{formatAttachmentMeta(attachment)}
								</Attachment.Description>
							{/if}
						</Attachment.Content>
						{#if attachmentState === 'error'}
							<Attachment.Actions>
								<Attachment.Action aria-label={text.retry} onclick={() => attachmentSource.retry(attachment.url)}>
									<RotateCwIcon />
								</Attachment.Action>
							</Attachment.Actions>
						{:else if attachment.source}
							{@const source = attachment.source}
							<Attachment.Actions>
								<Attachment.Action
									aria-label={text.downloadAttachment}
									onclick={() => startDownload(source, attachment.filename ?? '')}
								>
									<DownloadIcon />
								</Attachment.Action>
							</Attachment.Actions>
						{/if}
					{/if}
				</Attachment.Root>
			{/each}
			</Attachment.Group>
			{#if !bodyText && reactions.length > 0}
				{@render ownReactions(message, reactionAlign, nameWidthPixels)}
			{/if}
			{#if !bodyText}{@render timeStamp(message)}{/if}
		</div>
	{/if}
	{#if message.isError}
		<Bubble.Root variant="destructive" class="max-w-[min(80%,32rem)]">
			<Bubble.Content>{text.errorSummary}</Bubble.Content>
			<Bubble.Reactions align={reactionAlign}>
				<Popover.Root>
					<Popover.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-xs"
								aria-label={text.errorDetailsLabel}
								class="aria-expanded:text-destructive"
							>
								<InfoIcon />
							</Button>
						{/snippet}
					</Popover.Trigger>
					<Popover.Content class="max-w-sm">
						<Popover.Header>
							<Popover.Title class="text-sm">{text.errorDetailsTitle}</Popover.Title>
							<Popover.Description class="text-sm break-words whitespace-pre-wrap">
								{bodyText}
							</Popover.Description>
						</Popover.Header>
					</Popover.Content>
				</Popover.Root>
			</Bubble.Reactions>
			{@render timeStamp(message)}
		</Bubble.Root>
	{:else if bodyText}
		<Bubble.Root
			variant={mine ? 'default' : 'muted'}
			class={`max-w-[min(80%,32rem)] ${reactionSpacing}`}
		>
			<Bubble.Content>
				<div class="chat-markdown prose prose-sm dark:prose-invert max-w-none">
					<ChannelMessageBody
						source={applyCustomEmoji(bodyText, message.customEmoji, customEmoji.nameToURL)}
						mentionLabels={mentionLabelsOf(message.mentions, (externalID) =>
							nameByExternalID.get(externalID)
						)}
					/>
				</div>
			</Bubble.Content>
			{#if reactions.length > 0}
				{@render ownReactions(message, reactionAlign, nameWidthPixels)}
			{/if}
			{@render timeStamp(message)}
		</Bubble.Root>
		{#if firstLinkIn(bodyText)}
			<ChannelLinkPreview url={firstLinkIn(bodyText)} />
		{/if}
	{/if}
	{#if message.interaction}
		<div class="flex flex-wrap gap-2">
			{#each message.interaction.options as option (option.key)}
				<Button
					size="sm"
					variant={option.key === message.interaction.recommendedOptionKey ? 'default' : 'outline'}
					disabled={isSending}
					onclick={() => answerChoice(option.label)}
				>
					{option.label}
				</Button>
			{/each}
		</div>
	{/if}
{/snippet}

{#snippet timeStamp(message: ChannelMessage)}
	<div
		class="pointer-events-none absolute bottom-0 left-full ml-1.5 flex items-end gap-1.5 group-data-[align=end]/message:right-full group-data-[align=end]/message:left-auto group-data-[align=end]/message:mr-1.5 group-data-[align=end]/message:ml-0 group-data-[align=end]/message:flex-row-reverse @max-[56rem]/conversation:contents"
	>
		<time
			class="text-muted-foreground/70 pb-0.5 text-[11px] whitespace-nowrap tabular-nums @max-[56rem]/conversation:absolute @max-[56rem]/conversation:bottom-0.5 @max-[56rem]/conversation:left-full @max-[56rem]/conversation:ml-1.5 @max-[56rem]/conversation:pb-0 @max-[56rem]/conversation:group-data-[align=end]/message:right-full @max-[56rem]/conversation:group-data-[align=end]/message:left-auto @max-[56rem]/conversation:group-data-[align=end]/message:mr-1.5 @max-[56rem]/conversation:group-data-[align=end]/message:ml-0"
		>
			{clockTime(message.sentAt)}{#if message.editedAt}&nbsp;· {text.edited}{/if}
		</time>
	</div>
{/snippet}

{#snippet threadChip(message: ChannelMessage, thread: ThreadSummary)}
	<div class="group-data-[align=end]/message:self-end flex w-fit items-center gap-1.5">
		<CornerDownRightIcon class="text-border size-4 shrink-0" />
		<button
			type="button"
			onclick={() => openThread(message)}
			class="text-muted-foreground hover:bg-muted/60 hover:text-foreground flex w-fit items-center gap-2 rounded-full border py-1 pe-3 ps-1 text-xs transition-colors"
		>
			<PersonAvatarStack
				people={thread.participants.map((participant) => ({
					name: participant.name,
					email: participant.email,
					seed: participant.email || participant.id || participant.name,
					image: participant.avatarURL,
					memberID: participant.memberID,
					externalID: participant.externalID
				}))}
				class="-space-x-1.5"
				avatarClass="ring-background size-5 ring-2"
			/>
			<span class="text-primary font-medium">{thread.replyCount}{text.repliesSuffix}</span>
			<span>{relativeTime(thread.lastReplyAt)}</span>
		</button>
	</div>
{/snippet}

{#snippet senderAvatar(sender: ChannelParticipant)}
	<Message.Avatar>
		<PersonAvatar
			name={sender.name}
			email={sender.email ?? ''}
			seed={sender.email || sender.id || sender.name}
			image={sender.avatarURL ?? ''}
			memberID={sender.memberID ?? ''}
			externalID={sender.externalID ?? ''}
		/>

	</Message.Avatar>
{/snippet}

{#snippet messageRow(message: ChannelMessage, startsGroup: boolean, endsGroup: boolean, isInTimeline: boolean)}
	{@const replyChip = isInTimeline ? message.thread : undefined}
	{@const isUnsent = outgoing.hasFailed(message.id)}
	<MessageRow
		{message}
		mine={isMine(message)}
		{startsGroup}
		{endsGroup}
		senderName={showSenderNames ? message.sender.name : ''}
		{canChange}
		canReply={isInTimeline && !message.threadRootId}
		canDelete={isMine(message) || canModerate}
		canEdit={canEditMessage(message, isMine(message))}
		isSettled={!message.id.startsWith('pending-')}
		hasFooter={replyChip !== undefined || isUnsent}
		copyable={whatToCopy(
			messageTextBeside(
				message.text,
				(message.attachments ?? []).map((attachment) => attachment.url)
			),
			pictureAddressesOf(openableAttachments(message.attachments ?? [], openableAddressOf))
		)}
		pictures={messagePicturesOf(openableAttachments(message.attachments ?? [], openableAddressOf))}
		onReply={() => openThread(message)}
		onEdit={() => (isInTimeline ? conversationComposer : threadComposer)?.beginEdit(message)}
		onCopy={(wanted) => messageActions.copy(wanted)}
		onDelete={() => messageActions.askToDelete(message)}
		onReact={(glyph) => messageActions.reactWith(message, glyph)}
		onCapture={isInTimeline ? capture.begin : undefined}
	>
		{#snippet avatar()}
			{@render senderAvatar(message.sender)}
		{/snippet}
		{#snippet children({ nameWidthPixels })}
			<Bubble.Group class="w-full">
				{@render messageBody(message, nameWidthPixels)}
			</Bubble.Group>
		{/snippet}
		{#snippet footer()}
			{#if replyChip}{@render threadChip(message, replyChip)}{/if}
			{#if isUnsent}
				<FailedMessageActions onRetry={() => void sendAgain(message.id)} onDiscard={() => outgoing.discard(message.id)} />
			{/if}
		{/snippet}
	</MessageRow>
{/snippet}

{#snippet threadBody()}
	<div class="@container/conversation min-h-0 flex-1 overflow-y-auto">
		<div class="flex flex-col gap-4 px-4 py-8">
			{#if openThreadRoot}
				{@render messageRow(openThreadRoot, true, true, false)}
				{#each threadReplyGroups as group (group.id)}
					<Message.Group>
						{#each group.items as reply, index (reply.id)}
							{@render messageRow(reply, index === 0, index === group.items.length - 1, false)}
						{/each}
					</Message.Group>
				{/each}
			{/if}
		</div>
	</div>
	{#key cacheGeneration}<ChannelComposer
		bind:this={threadComposer}
		bind:isSending={isThreadSending}
		bind:editing={threadEditing}
		name="thread"
		placeholder={text.threadComposerPlaceholder}
		{participants}
		{isGroup}
		cancelsEditOnEscape={threadLayout === 'inline'}
		saveEdit={messageActions.saveEdit}
		onSend={sendThreadReply}
	/>{/key}
{/snippet}

<div class="flex min-h-0 flex-1">
<div class="relative flex min-h-0 min-w-0 flex-1 flex-col" style:--dock-height="{dockHeight}px">
	<div class="min-h-0 min-w-0 flex-1 overflow-hidden">
			{#if !hasLoadedOnce}
				<ChannelLoadingSkeleton label={text.loadingMessages} />
		{:else if loadFailed && shownMessages.length === 0}
			<div class="flex h-full items-center justify-center px-6 pb-[var(--dock-height)]">
				<div role="alert" class="grid max-w-sm gap-3 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm">
					<p class="font-medium text-destructive">{text.unavailableTitle}</p>
					<p class="text-muted-foreground">{text.unavailableDescription}</p>
					<Button variant="outline" size="sm" onclick={loadConversation}>{text.retry}</Button>
				</div>
			</div>
		{:else if shownMessages.length === 0}
			<Empty.Root class="h-full pb-[var(--dock-height)]">
				<Empty.Header>
					<Empty.Media variant="icon"><MessageCircleDashedIcon /></Empty.Media>
					<Empty.Title>{text.emptyTitle}</Empty.Title>
					<Empty.Description>{text.emptyDescription}</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<div class="relative flex h-full flex-col">
				{#if isLoadingOlder}
					<div class="pointer-events-none absolute inset-x-0 top-2 z-10 flex justify-center">
						<span
							class="bg-background/80 text-muted-foreground inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs shadow-sm backdrop-blur"
							role="status"
						>
							<Spinner class="size-3" />
							{text.loadingOlder}
						</span>
					</div>
				{/if}
				<div
					bind:this={scrollContainer}
					onscroll={handleViewportScroll}
					use:chooseMessageOnClick={{ enabled: capture.isCapturing, onChoose: capture.choose }}
					class="@container/conversation isolate flex min-h-0 flex-1 flex-col-reverse gap-4 overflow-x-hidden overflow-y-auto overscroll-y-none px-4 pt-12 pb-[calc(var(--dock-height)+2.5rem)] [scrollbar-gutter:stable]"
				>
					{#each reversedTimeline as item (item.id)}
						{#if item.kind === 'date'}
							<Marker.Root variant="separator">
								<Marker.Content>{item.label}</Marker.Content>
							</Marker.Root>
						{:else}
							<Message.Group>
								{#each item.items as message, index (message.id)}
									{@render messageRow(message, index === 0, index === item.items.length - 1, true)}
								{/each}
							</Message.Group>
						{/if}
					{/each}
				</div>
				{#if capture.isCapturing}
					<MessageCaptureOverlay scroller={scrollContainer} firstID={capture.firstID} lastID={capture.lastID} />
				{/if}
				{#if showScrollToBottom && !capture.isCapturing}
					<Button
						variant="outline"
						size="sm"
						onclick={scrollToBottom}
						class="absolute bottom-[calc(var(--dock-height)+2.5rem)] left-1/2 z-10 -translate-x-1/2 shadow-md"
					>
						<ArrowDownIcon data-icon="inline-start" />
						{jumpToLatestLabel(unseenCount, text)}
					</Button>
				{/if}
			</div>
		{/if}
	</div>
	<ConversationDock {activity} bind:height={dockHeight}>
		{#if capture.isCapturing}
			<MessageCaptureBar
				scroller={scrollContainer}
				chosenIDs={capture.chosenIDs}
				onClearChoice={capture.clearChoice}
				onCancel={capture.end}
				onDone={capture.end}
			/>
		{/if}
		<div class={capture.isCapturing ? 'hidden' : 'contents'}>
		{#key cacheGeneration}
		<ChannelComposer
			bind:this={conversationComposer}
			bind:isSending
			name="conversation"
			placeholder={text.composerPlaceholder}
			{participants}
			{isGroup}
			cancelsEditOnEscape={true}
			saveEdit={messageActions.saveEdit}
			onSend={sendToConversation}
			onTyping={typing.announce}
			onCapture={capture.begin}
		/>
		{/key}
		</div>
	</ConversationDock>
</div>
{#if threadLayout === 'inline' && openThreadRoot}
	<aside class="flex min-h-0 w-full max-w-md flex-col border-l">
		<header class="flex items-center justify-between gap-2 border-b p-3">
			<div class="flex flex-col gap-0.5">
				<div class="flex items-center gap-2 text-sm font-medium">
					<MessageSquareIcon class="size-4" />
					{text.threadTitle}
				</div>
				{#if openThreadRoot.thread}
					<span class="text-muted-foreground text-xs">
						{openThreadRoot.thread.replyCount}{text.repliesSuffix}
					</span>
				{/if}
			</div>
			<Button
				variant="ghost"
				size="icon-sm"
				aria-label={text.closeThread}
				onclick={closeThread}
			>
				<XIcon />
			</Button>
		</header>
		{@render threadBody()}
	</aside>
{/if}
</div>

<svelte:document onvisibilitychange={() => (isPageVisible = document.visibilityState === 'visible')} />
<ChannelLightbox bind:view={lightbox} />

{#if threadLayout === 'sheet'}
	<Sheet.Root
		open={!!openThreadRoot}
		onOpenChange={(open) => {
			if (!open) closeThread();
		}}
	>
		<Sheet.Content
			side="right"
			class="flex w-full flex-col gap-0 p-0 sm:max-w-md"
			onEscapeKeydown={(event) => {
				if (!threadEditing) return;
				event.preventDefault();
				threadComposer?.cancelEdit();
			}}
		>
			<Sheet.Header class="border-b">
				<Sheet.Title>{text.threadTitle}</Sheet.Title>
			</Sheet.Header>
			{@render threadBody()}
		</Sheet.Content>
	</Sheet.Root>
{/if}

<style>
	.chat-markdown {
		overflow-wrap: anywhere;
	}
	.chat-markdown,
	.chat-markdown :global(*) {
		color: inherit;
	}
	.chat-markdown :global(p:first-child) {
		margin-top: 0;
	}
	.chat-markdown :global(p:last-child) {
		margin-bottom: 0;
	}
	.chat-markdown :global(table) {
		display: block;
		max-width: 100%;
		overflow-x: auto;
		border-collapse: collapse;
		margin: 0.5rem 0;
		font-size: 0.9em;
		border: 1px solid currentColor;
	}
	.chat-markdown :global(th),
	.chat-markdown :global(td) {
		border: 1px solid currentColor;
		padding: 0.375rem 0.625rem;
		text-align: left;
		white-space: nowrap;
	}
	.chat-markdown :global(thead th) {
		background: color-mix(in srgb, currentColor 12%, transparent);
		font-weight: 600;
	}
	.chat-markdown :global(tbody tr:nth-child(even)) {
		background: color-mix(in srgb, currentColor 6%, transparent);
	}
	.chat-markdown :global(img) {
		display: inline-block;
		height: 1.4em;
		width: auto;
		margin: 0;
		vertical-align: text-bottom;
	}
</style>
