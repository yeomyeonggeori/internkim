<script lang="ts">
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import * as Marker from '$lib/components/ui/marker/index.js';
	import * as Message from '$lib/components/ui/message/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import ChannelCode from './channel-code.svelte';
	import ChannelLinkPreview from './channel-link-preview.svelte';
	import MessageReactions from './message-reactions.svelte';
	import MessageRow from './message-row.svelte';
	import { firstLinkIn } from './channel-link';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		canChangeMessages,
		fetchChannelConversation,
		messageTextBeside,
		applyCustomEmoji,
		sendChannelMessage,
		type ChannelMessage,
		type ChannelOutgoingAttachment,
		type ChannelParticipant,
		type ThreadSummary
	} from './channel-api';
	import {
		fileToAttachment,
		formatAttachmentMeta,
		openableAttachments,
		pictureAddressesOf
	} from './channel-attachments';
	import { messageActionsFor } from './channel-message-actions';
	import { whatToCopy } from './message-copy';
	import { messagesWithReactions } from './channel-reactions';
	import { getCachedMessages, getCachedReaderID, setCachedMessages, setCachedReaderID } from './channel-message-cache';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import CornerDownRightIcon from '@lucide/svelte/icons/corner-down-right';
	import FileIcon from '@lucide/svelte/icons/file';
	import InfoIcon from '@lucide/svelte/icons/info';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import MessageCircleDashedIcon from '@lucide/svelte/icons/message-circle-dashed';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { customEmoji } from '$lib/stores/custom-emoji.svelte';
	import { attachmentSource } from '$lib/stores/attachment-source.svelte';
	import { onCompanyEvent } from '$lib/host-bridge';
	import type { CompanyEvent } from '$lib/company-event';
	import { isSupabaseConfigured } from '$lib/supabase';
	import { onDestroy, onMount, type Snippet } from 'svelte';
	import { fade, scale } from 'svelte/transition';

	let { isActive = true, threadLayout = 'sheet', channelId, showSenderNames = false, canModerate = false }: {
		isActive?: boolean;
		threadLayout?: 'sheet' | 'inline';
		channelId?: string;
		showSenderNames?: boolean;
		canModerate?: boolean;
	} = $props();

	const text = createPageText(channelText);
	const idleRefreshIntervalMs = isSupabaseConfigured() ? 30_000 : 5000;
	const workingRefreshIntervalMs = 1500;

	type PendingAttachment = {
		id: string;
		previewURL: string;
		isImage: boolean;
		sizeBytes: number;
		attachment: ChannelOutgoingAttachment;
	};

	// ponytail: 전환 중 메시지 입력 임시 잠금 (되돌리려면 false)
	const messageInputDisabled = false;
	let messages = $state<ChannelMessage[]>([]);
	let currentUserID = $state(getCachedReaderID());
	let currentUserEmail = $state('');
	let currentUserImage = $state('');
	let isAgentWorking = $state(false);
	let composerValue = $state('');
	let isSending = $state(false);
	let loadFailed = $state(false);
	let hasLoadedOnce = $state(false);
	let olderMessages = $state<ChannelMessage[]>([]);
	let hasMoreBefore = $state(false);
	let historyCursor = $state('');
	let isLoadingOlder = $state(false);
	let lightbox = $state<{ images: string[]; index: number } | null>(null);
	let lightboxTouchStartX = 0;
	let lightboxTouchMoved = false;
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;

	function openLightbox(images: string[], index: number) {
		if (images.length === 0) return;
		lightbox = { images, index: Math.max(0, index) };
	}

	function stepLightbox(delta: number) {
		if (!lightbox || lightbox.images.length < 2) return;
		const count = lightbox.images.length;
		lightbox = { images: lightbox.images, index: (lightbox.index + delta + count) % count };
	}

	function handleLightboxKeydown(event: KeyboardEvent) {
		if (!lightbox) return;
		if (event.key === 'Escape') lightbox = null;
		else if (event.key === 'ArrowRight') stepLightbox(1);
		else if (event.key === 'ArrowLeft') stepLightbox(-1);
	}

	function handleLightboxTouchStart(event: TouchEvent) {
		lightboxTouchStartX = event.changedTouches[0]?.clientX ?? 0;
		lightboxTouchMoved = false;
	}

	function handleLightboxTouchEnd(event: TouchEvent) {
		const deltaX = (event.changedTouches[0]?.clientX ?? 0) - lightboxTouchStartX;
		if (Math.abs(deltaX) < 40) return;
		lightboxTouchMoved = true;
		stepLightbox(deltaX < 0 ? 1 : -1);
	}

	function closeLightboxFromBackdrop() {
		if (lightboxTouchMoved) {
			lightboxTouchMoved = false;
			return;
		}
		lightbox = null;
	}
	let lastConversationSignature = '';
	let pendingAttachments = $state<PendingAttachment[]>([]);
	let fileInput = $state<HTMLInputElement | null>(null);
	let attachmentSerial = 0;
	let openThreadRoot = $state<ChannelMessage | null>(null);
	let threadComposer = $state('');
	let isThreadSending = $state(false);
	let threadPendingAttachments = $state<PendingAttachment[]>([]);
	let threadFileInput = $state<HTMLInputElement | null>(null);

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

	const threadRepliesByRoot = $derived.by(() => {
		const repliesByRoot = new Map<string, ChannelMessage[]>();
		for (const message of messages) {
			if (!message.threadRootId) continue;
			const replies = repliesByRoot.get(message.threadRootId) ?? [];
			replies.push(message);
			repliesByRoot.set(message.threadRootId, replies);
		}
		for (const replies of repliesByRoot.values()) {
			replies.sort((first, second) => first.sentAt.localeCompare(second.sentAt));
		}
		return repliesByRoot;
	});

	const visibleMessages = $derived.by(() => {
		const presentIds = new Set(messages.map((message) => message.id));
		return messages
			.filter((message) => !message.threadRootId || !presentIds.has(message.threadRootId))
			.map((message) => {
				const replies = threadRepliesByRoot.get(message.id);
				if (!replies || replies.length === 0) return message;
				return { ...message, thread: threadSummaryFor(message, replies) };
			});
	});

	function threadSummaryFor(rootMessage: ChannelMessage, replies: ChannelMessage[]): ThreadSummary {
		const participantsById = new Map<string, ChannelParticipant>();
		for (const message of [rootMessage, ...replies]) {
			participantsById.set(message.sender.id, message.sender);
		}
		return {
			replyCount: replies.length,
			lastReplyAt: replies.at(-1)?.sentAt ?? rootMessage.sentAt,
			participants: [...participantsById.values()]
		};
	}

	const messageGroups = $derived.by(() => {
		const groups: { id: string; senderID: string; items: ChannelMessage[] }[] = [];
		for (const message of visibleMessages) {
			const lastGroup = groups.at(-1);
			const canMergeIntoLastGroup =
				lastGroup &&
				lastGroup.senderID === message.sender.id &&
				!lastGroup.items.at(-1)?.thread;
			if (canMergeIntoLastGroup) lastGroup.items.push(message);
			else groups.push({ id: message.id, senderID: message.sender.id, items: [message] });
		}
		return groups;
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
		openThreadRoot ? (threadRepliesByRoot.get(openThreadRoot.id) ?? []) : []
	);

	async function loadConversation() {
		try {
			const conversation = await fetchChannelConversation(channelId);
			currentUserID = conversation.currentUserID;
			setCachedReaderID(conversation.currentUserID);
			const latestIncoming = conversation.messages.at(-1);
			if (latestIncoming && !isMine(latestIncoming) && latestIncoming.id !== messages.at(-1)?.id) {
				isAgentWorking = false;
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
				setCachedMessages(channelId, conversation.messages);
				if (openThreadRoot) {
					openThreadRoot = visibleMessages.find((message) => message.id === openThreadRoot?.id) ?? openThreadRoot;
				}
			}
			loadFailed = false;
		} catch {
			loadFailed = true;
		} finally {
			hasLoadedOnce = true;
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

	const olderPrefetchScreens = 3;

	function distanceFromOldestTop(container: HTMLElement): number {
		return container.scrollHeight - container.clientHeight - Math.abs(container.scrollTop);
	}

	async function loadOlderMessages() {
		if (isLoadingOlder || !hasMoreBefore || !historyCursor) return;
		isLoadingOlder = true;
		try {
			const page = await fetchChannelConversation(channelId, historyCursor);
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
			isLoadingOlder = false;
		}
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
		try {
			const response = await fetch('/auth/session', { credentials: 'include', cache: 'no-store' });
			if (!response.ok) return;
			const session: { authenticated?: boolean; email?: string; image?: string } = await response.json();
			if (session.authenticated) {
				currentUserEmail = session.email ?? '';
				currentUserImage = session.image ?? '';
			}
		} catch {
			currentUserEmail = '';
			currentUserImage = '';
		}
	}

	function relativeTime(isoTimestamp: string): string {
		const elapsedSeconds = Math.max(0, Math.round((Date.now() - new Date(isoTimestamp).getTime()) / 1000));
		if (elapsedSeconds < 60) return '방금';
		const elapsedMinutes = Math.round(elapsedSeconds / 60);
		if (elapsedMinutes < 60) return `${elapsedMinutes}분 전`;
		const elapsedHours = Math.round(elapsedMinutes / 60);
		if (elapsedHours < 24) return `${elapsedHours}시간 전`;
		return `${Math.round(elapsedHours / 24)}일 전`;
	}

	function clockTime(isoTimestamp: string): string {
		return new Date(isoTimestamp).toLocaleTimeString('ko-KR', {
			hour: 'numeric',
			minute: '2-digit',
			hour12: true
		});
	}

	function dateKeyOf(isoTimestamp: string): string {
		return new Date(isoTimestamp).toLocaleDateString('en-CA');
	}

	function dateLabel(isoTimestamp: string): string {
		return new Date(isoTimestamp).toLocaleDateString('ko-KR', {
			year: 'numeric',
			month: 'long',
			day: 'numeric',
			weekday: 'short'
		});
	}

	function openThread(rootMessage: ChannelMessage) {
		openThreadRoot = rootMessage;
	}

	async function submitThreadReply(event: SubmitEvent) {
		event.preventDefault();
		const trimmedReply = threadComposer.trim();
		const outgoingAttachments = threadPendingAttachments.map((pending) => pending.attachment);
		if ((!trimmedReply && outgoingAttachments.length === 0) || isThreadSending || !openThreadRoot) return;
		isThreadSending = true;
		threadComposer = '';
		clearThreadAttachments();
		try {
			await sendChannelMessage(trimmedReply, outgoingAttachments, channelId, openThreadRoot.id);
			await loadConversation();
		} catch {
			loadFailed = true;
		} finally {
			isThreadSending = false;
		}
	}

	function handleThreadKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
		event.preventDefault();
		if (!(event.currentTarget instanceof HTMLElement)) return;
		event.currentTarget.closest('form')?.requestSubmit();
	}

	function scheduleRefresh() {
		clearTimeout(refreshTimer);
		if (!isActive) return;
		const delay = isAgentWorking ? workingRefreshIntervalMs : idleRefreshIntervalMs;
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

	async function submitMessage(event: SubmitEvent) {
		event.preventDefault();
		const trimmedMessage = composerValue.trim();
		const outgoingAttachments = pendingAttachments.map((pending) => pending.attachment);
		if ((!trimmedMessage && outgoingAttachments.length === 0) || isSending) return;
		isSending = true;
		composerValue = '';
		const attachmentSummary = pendingAttachments.map((pending) => pending.attachment.filename).join(', ');
		clearAttachments();
		messages = [
			...messages,
			{
				id: `pending-${messages.length}`,
				sender: currentUser,
				text: trimmedMessage || attachmentSummary,
				sentAt: new Date().toISOString()
			}
		];
		scrollContainer?.scrollTo({ top: 0 });
		try {
			await sendChannelMessage(trimmedMessage, outgoingAttachments, channelId);
			isAgentWorking = true;
			await loadConversation();
		} catch {
			loadFailed = true;
		} finally {
			isSending = false;
			scheduleRefresh();
		}
	}

	function filesFromInput(event: Event): File[] {
		if (!(event.currentTarget instanceof HTMLInputElement)) return [];
		const files = Array.from(event.currentTarget.files ?? []);
		event.currentTarget.value = '';
		return files;
	}

	async function buildPendingAttachments(files: File[]): Promise<PendingAttachment[]> {
		const built: PendingAttachment[] = [];
		for (const file of files) {
			built.push({
				id: `attachment-${attachmentSerial++}`,
				previewURL: URL.createObjectURL(file),
				isImage: file.type.startsWith('image/'),
				sizeBytes: file.size,
				attachment: await fileToAttachment(file)
			});
		}
		return built;
	}

	function withoutAttachment(list: PendingAttachment[], id: string): PendingAttachment[] {
		const removed = list.find((pending) => pending.id === id);
		if (removed) URL.revokeObjectURL(removed.previewURL);
		return list.filter((pending) => pending.id !== id);
	}

	function revokeAttachments(list: PendingAttachment[]): PendingAttachment[] {
		for (const pending of list) URL.revokeObjectURL(pending.previewURL);
		return [];
	}

	async function handleFilesSelected(event: Event) {
		pendingAttachments = [...pendingAttachments, ...(await buildPendingAttachments(filesFromInput(event)))];
	}

	function removeAttachment(id: string) {
		pendingAttachments = withoutAttachment(pendingAttachments, id);
	}

	function clearAttachments() {
		pendingAttachments = revokeAttachments(pendingAttachments);
	}

	async function handleThreadFilesSelected(event: Event) {
		threadPendingAttachments = [
			...threadPendingAttachments,
			...(await buildPendingAttachments(filesFromInput(event)))
		];
	}

	function removeThreadAttachment(id: string) {
		threadPendingAttachments = withoutAttachment(threadPendingAttachments, id);
	}

	function clearThreadAttachments() {
		threadPendingAttachments = revokeAttachments(threadPendingAttachments);
	}

	function closeThread() {
		clearThreadAttachments();
		threadComposer = '';
		openThreadRoot = null;
	}

	async function answerChoice(optionLabel: string) {
		if (isSending) return;
		isSending = true;
		try {
			await sendChannelMessage(optionLabel, [], channelId);
			isAgentWorking = true;
			await loadConversation();
		} catch {
			loadFailed = true;
		} finally {
			isSending = false;
			scheduleRefresh();
		}
	}

	function handleComposerKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
		event.preventDefault();
		if (!(event.currentTarget instanceof HTMLElement)) return;
		event.currentTarget.closest('form')?.requestSubmit();
	}

	$effect(() => {
		if (isActive) scheduleRefresh();
		else clearTimeout(refreshTimer);
	});

	$effect(() => {
		const activeChannelID = channelId;
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

	onMount(async () => {
		customEmoji.load();
		if (isSupabaseConfigured()) stopListeningForArrivals = onCompanyEvent((event) => void readAgainOnArrival(event));
		await loadCurrentUser();
		scheduleRefresh();
	});

	onDestroy(() => {
		stopListeningForArrivals();
		clearTimeout(refreshTimer);
		clearAttachments();
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
		onPick={(glyph) => messageActions.reactWith(message, glyph)}
	/>
{/snippet}

{#snippet messageBody(message: ChannelMessage, nameWidthPixels: number, hasFooter: boolean, isToolbarShown: boolean, toolbar: Snippet)}
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
	{@const imageAttachmentURLs = pictureAddressesOf(attachments)}
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
				class="block cursor-zoom-in overflow-hidden rounded-lg"
				aria-label={loneImage.filename ?? '이미지 크게 보기'}
				onclick={() => openLightbox(imageAttachmentURLs, 0)}
			>
				<img
					data-message-picture
					src={loneImage.source}
					alt={loneImage.filename ?? ''}
					width={loneImage.widthPixels}
					height={loneImage.heightPixels}
					loading="lazy"
					decoding="async"
					class="h-auto max-h-[min(60vh,26rem)] w-auto max-w-full rounded-lg object-contain"
				/>
			</button>
			{#if !bodyText && reactions.length > 0}
				{@render ownReactions(message, reactionAlign, nameWidthPixels)}
			{/if}
			{#if !bodyText}{@render timeStamp(message, hasFooter, isToolbarShown, toolbar)}{/if}
		</div>
	{:else if attachments.length > 0}
		<div class={`relative w-fit max-w-[80%] self-start group-data-[align=end]/message:self-end ${imageReactionSpacing}`}>
			<Attachment.Group class="relative w-fit max-w-full">
			{#each attachments as attachment (attachment.url)}
				<Attachment.Root orientation="vertical">
					{#if attachment.kind === 'image' && attachment.source}
						<Attachment.Media variant="image">
							<button
								type="button"
								class="block h-full w-full cursor-zoom-in"
								aria-label={attachment.filename ?? '이미지 크게 보기'}
								onclick={() =>
									openLightbox(imageAttachmentURLs, imageAttachmentURLs.indexOf(attachment.source ?? ''))}
							>
								<img
									data-message-picture
									src={attachment.source}
									alt={attachment.filename ?? ''}
									loading="lazy"
									decoding="async"
									class="aspect-square h-full w-full object-cover"
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
							{#if formatAttachmentMeta(attachment)}
								<Attachment.Description>
									{formatAttachmentMeta(attachment)}
								</Attachment.Description>
							{/if}
						</Attachment.Content>
					{/if}
				</Attachment.Root>
			{/each}
			</Attachment.Group>
			{#if !bodyText && reactions.length > 0}
				{@render ownReactions(message, reactionAlign, nameWidthPixels)}
			{/if}
			{#if !bodyText}{@render timeStamp(message, hasFooter, isToolbarShown, toolbar)}{/if}
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
			{@render timeStamp(message, hasFooter, isToolbarShown, toolbar)}
		</Bubble.Root>
	{:else if bodyText}
		<Bubble.Root
			variant={mine ? 'default' : 'muted'}
			class={`max-w-[min(80%,32rem)] ${reactionSpacing}`}
		>
			<Bubble.Content>
				<div class="chat-markdown prose prose-sm dark:prose-invert max-w-none">
					<SvelteMarkdown
						source={applyCustomEmoji(bodyText, message.customEmoji, customEmoji.nameToURL)}
						options={{ breaks: true, gfm: true }}
						renderers={{ code: ChannelCode }}
					/>
				</div>
			</Bubble.Content>
			{#if reactions.length > 0}
				{@render ownReactions(message, reactionAlign, nameWidthPixels)}
			{/if}
			{@render timeStamp(message, hasFooter, isToolbarShown, toolbar)}
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

{#snippet timeStamp(message: ChannelMessage, hasFooter: boolean, isToolbarShown: boolean, toolbar: Snippet)}
	<div
		class="pointer-events-none absolute bottom-0 left-full ml-1.5 flex items-end gap-1.5 group-data-[align=end]/message:right-full group-data-[align=end]/message:left-auto group-data-[align=end]/message:mr-1.5 group-data-[align=end]/message:ml-0 group-data-[align=end]/message:flex-row-reverse @max-[56rem]/conversation:contents"
	>
		<time
			class="text-muted-foreground/70 pb-0.5 text-[11px] whitespace-nowrap tabular-nums @max-[56rem]/conversation:absolute @max-[56rem]/conversation:bottom-0.5 @max-[56rem]/conversation:left-full @max-[56rem]/conversation:ml-1.5 @max-[56rem]/conversation:pb-0 @max-[56rem]/conversation:group-data-[align=end]/message:right-full @max-[56rem]/conversation:group-data-[align=end]/message:left-auto @max-[56rem]/conversation:group-data-[align=end]/message:mr-1.5 @max-[56rem]/conversation:group-data-[align=end]/message:ml-0"
		>
			{clockTime(message.sentAt)}
		</time>
		{#if isToolbarShown}
			<div
				class={`bg-background pointer-events-auto z-20 rounded-lg border p-0.5 shadow-sm @max-[56rem]/conversation:absolute @max-[56rem]/conversation:left-0 @max-[56rem]/conversation:group-data-[align=end]/message:right-0 @max-[56rem]/conversation:group-data-[align=end]/message:left-auto ${
					hasFooter
						? '@max-[56rem]/conversation:bottom-full @max-[56rem]/conversation:mb-1'
						: '@max-[56rem]/conversation:top-full @max-[56rem]/conversation:mt-1'
				}`}
			>
				{@render toolbar()}
			</div>
		{/if}
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
	<MessageRow
		{message}
		mine={isMine(message)}
		{startsGroup}
		{endsGroup}
		senderName={showSenderNames ? message.sender.name : ''}
		{canChange}
		canReply={isInTimeline && !message.threadRootId}
		canDelete={isMine(message) || canModerate}
		isSettled={!message.id.startsWith('pending-')}
		hasFooter={replyChip !== undefined}
		copyable={whatToCopy(
			messageTextBeside(
				message.text,
				(message.attachments ?? []).map((attachment) => attachment.url)
			),
			pictureAddressesOf(openableAttachments(message.attachments ?? [], openableAddressOf))
		)}
		onReply={() => openThread(message)}
		onCopy={(wanted) => messageActions.copy(wanted)}
		onDelete={() => messageActions.askToDelete(message)}
		onReact={(glyph) => messageActions.reactWith(message, glyph)}
	>
		{#snippet avatar()}
			{@render senderAvatar(message.sender)}
		{/snippet}
		{#snippet children({ nameWidthPixels, hasFooter, isToolbarShown, toolbar })}
			<Bubble.Group class="w-full">
				{@render messageBody(message, nameWidthPixels, hasFooter, isToolbarShown, toolbar)}
			</Bubble.Group>
		{/snippet}
		{#snippet footer()}
			{#if replyChip}{@render threadChip(message, replyChip)}{/if}
		{/snippet}
	</MessageRow>
{/snippet}

{#snippet threadBody()}
	<div class="@container/conversation min-h-0 flex-1 overflow-y-auto">
		<div class="flex flex-col gap-4 px-4 py-8">
			{#if openThreadRoot}
				{@render messageRow(openThreadRoot, true, true, false)}
				{#each threadReplies as reply (reply.id)}
					{@render messageRow(reply, true, true, false)}
				{/each}
			{/if}
		</div>
	</div>
	<form onsubmit={submitThreadReply} class="border-t p-3">
		<input bind:this={threadFileInput} type="file" multiple class="hidden" onchange={handleThreadFilesSelected} />
		{#if threadPendingAttachments.length > 0}
			<Attachment.Group class="mb-2">
				{#each threadPendingAttachments as pending (pending.id)}
					<Attachment.Root size="sm">
						{#if pending.isImage}
							<Attachment.Media variant="image">
								<img src={pending.previewURL} alt="" />
							</Attachment.Media>
						{:else}
							<Attachment.Media>
								<FileIcon />
							</Attachment.Media>
						{/if}
						<Attachment.Content>
							<Attachment.Title>{pending.attachment.filename}</Attachment.Title>
							<Attachment.Description>
								{formatAttachmentMeta({
									mimeType: pending.attachment.contentType,
									filename: pending.attachment.filename,
									sizeBytes: pending.sizeBytes
								})}
							</Attachment.Description>
						</Attachment.Content>
						<Attachment.Actions>
							<Attachment.Action
								aria-label={text.removeAttachment}
								onclick={() => removeThreadAttachment(pending.id)}
							>
								<XIcon />
							</Attachment.Action>
						</Attachment.Actions>
					</Attachment.Root>
				{/each}
			</Attachment.Group>
		{/if}
		<InputGroup.Root>
			<InputGroup.Textarea
				bind:value={threadComposer}
				placeholder={messageInputDisabled ? text.composerDisabledPlaceholder : text.threadComposerPlaceholder}
				aria-label={text.threadComposerPlaceholder}
				rows={1}
				onkeydown={handleThreadKeydown}
				disabled={messageInputDisabled}
			/>
			<InputGroup.Addon align="block-end" class="pt-1">
				<InputGroup.Button
					type="button"
					variant="outline"
					size="icon-sm"
					aria-label={text.addAttachment}
					onclick={() => threadFileInput?.click()}
					disabled={messageInputDisabled}
				>
					<PlusIcon />
				</InputGroup.Button>
				<InputGroup.Button
					type="submit"
					variant="default"
					size="icon-sm"
					class="ms-auto"
					disabled={messageInputDisabled || (threadComposer.trim().length === 0 && threadPendingAttachments.length === 0) || isThreadSending}
				>
					<ArrowUpIcon />
					<span class="sr-only">{text.send}</span>
				</InputGroup.Button>
			</InputGroup.Addon>
		</InputGroup.Root>
	</form>
{/snippet}

<div class="flex min-h-0 flex-1">
<div class="flex min-h-0 min-w-0 flex-1 flex-col">
	<div class="min-h-0 min-w-0 flex-1 overflow-hidden">
		{#if !hasLoadedOnce}
			<div class="flex h-full flex-col gap-8 px-4 py-12">
				{#each Array(6) as _, index (index)}
					<div class="flex gap-3" class:flex-row-reverse={index % 3 === 0}>
						<Skeleton class="size-9 shrink-0 rounded-full" />
						<div class="flex max-w-[70%] flex-col gap-2">
							<Skeleton class="h-4 w-24" />
							<Skeleton class="h-16 w-64 max-w-full rounded-2xl" />
						</div>
					</div>
				{/each}
			</div>
		{:else if loadFailed && messages.length === 0}
			<Empty.Root class="h-full">
				<Empty.Header>
					<Empty.Media variant="icon"><MessageCircleDashedIcon /></Empty.Media>
					<Empty.Title>{text.unavailableTitle}</Empty.Title>
					<Empty.Description>{text.unavailableDescription}</Empty.Description>
				</Empty.Header>
				<Button variant="outline" size="sm" onclick={loadConversation}>{text.retry}</Button>
			</Empty.Root>
		{:else if messages.length === 0}
			<Empty.Root class="h-full">
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
							class="bg-background/80 text-muted-foreground rounded-full px-3 py-1 text-xs shadow-sm backdrop-blur"
						>
							{text.loadingOlder}
						</span>
					</div>
				{/if}
				<div
					bind:this={scrollContainer}
					onscroll={handleViewportScroll}
					class="@container/conversation flex min-h-0 flex-1 flex-col-reverse gap-4 overflow-x-hidden overflow-y-auto overscroll-y-none px-4 py-12 [scrollbar-gutter:stable]"
				>
					{#if isAgentWorking}
						<Marker.Root role="status">
							<Marker.Content class="shimmer">{text.working}</Marker.Content>
						</Marker.Root>
					{/if}
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
				{#if showScrollToBottom}
					<button
						type="button"
						aria-label="맨 아래로"
						onclick={scrollToBottom}
						class="bg-background hover:bg-muted text-foreground absolute bottom-4 left-1/2 flex size-9 -translate-x-1/2 items-center justify-center rounded-full border shadow-md transition"
					>
						<ChevronDownIcon class="size-5" />
					</button>
				{/if}
			</div>
		{/if}
	</div>
	<form onsubmit={submitMessage} class="border-t p-3">
		<input bind:this={fileInput} type="file" multiple class="hidden" onchange={handleFilesSelected} />
		{#if pendingAttachments.length > 0}
			<Attachment.Group class="mb-2">
				{#each pendingAttachments as pending (pending.id)}
					<Attachment.Root size="sm">
						{#if pending.isImage}
							<Attachment.Media variant="image">
								<img src={pending.previewURL} alt="" />
							</Attachment.Media>
						{:else}
							<Attachment.Media>
								<FileIcon />
							</Attachment.Media>
						{/if}
						<Attachment.Content>
							<Attachment.Title>{pending.attachment.filename}</Attachment.Title>
							<Attachment.Description>
								{formatAttachmentMeta({
									mimeType: pending.attachment.contentType,
									filename: pending.attachment.filename,
									sizeBytes: pending.sizeBytes
								})}
							</Attachment.Description>
						</Attachment.Content>
						<Attachment.Actions>
							<Attachment.Action
								aria-label={text.removeAttachment}
								onclick={() => removeAttachment(pending.id)}
							>
								<XIcon />
							</Attachment.Action>
						</Attachment.Actions>
					</Attachment.Root>
				{/each}
			</Attachment.Group>
		{/if}
		<InputGroup.Root>
			<InputGroup.Textarea
				bind:value={composerValue}
				placeholder={messageInputDisabled ? text.composerDisabledPlaceholder : text.composerPlaceholder}
				aria-label={text.composerPlaceholder}
				rows={2}
				onkeydown={handleComposerKeydown}
				disabled={messageInputDisabled}
			/>
			<InputGroup.Addon align="block-end" class="pt-1">
				<InputGroup.Button
					type="button"
					variant="outline"
					size="icon-sm"
					aria-label={text.addAttachment}
					onclick={() => fileInput?.click()}
					disabled={messageInputDisabled}
				>
					<PlusIcon />
				</InputGroup.Button>
				<InputGroup.Button
					type="submit"
					variant="default"
					size="icon-sm"
					class="ms-auto"
					disabled={messageInputDisabled || (composerValue.trim().length === 0 && pendingAttachments.length === 0) || isSending}
				>
					<ArrowUpIcon />
					<span class="sr-only">{text.send}</span>
				</InputGroup.Button>
			</InputGroup.Addon>
		</InputGroup.Root>
	</form>
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

<svelte:window onkeydown={handleLightboxKeydown} />
{#if lightbox}
	{@const currentImage = lightbox.images[lightbox.index]}
	{@const hasMultiple = lightbox.images.length > 1}
	<div
		class="fixed inset-0 z-(--layer-alert) flex items-center justify-center"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		transition:fade={{ duration: 150 }}
		ontouchstart={handleLightboxTouchStart}
		ontouchend={handleLightboxTouchEnd}
	>
		<button
			type="button"
			class="absolute inset-0 cursor-zoom-out bg-black/80"
			aria-label="이미지 닫기"
			onclick={closeLightboxFromBackdrop}
		></button>
		{#key lightbox.index}
			<img
				src={currentImage}
				alt=""
				class="pointer-events-none relative z-10 max-h-full max-w-full rounded-md object-contain p-6"
				in:scale={{ duration: 200, start: 0.94 }}
			/>
		{/key}
		{#if hasMultiple}
			<button
				type="button"
				class="absolute left-3 z-20 flex size-11 items-center justify-center rounded-full bg-white/10 text-white backdrop-blur transition hover:bg-white/20 max-md:hidden"
				aria-label="이전 이미지"
				onclick={() => stepLightbox(-1)}
			>
				<ChevronLeftIcon class="size-6" />
			</button>
			<button
				type="button"
				class="absolute right-3 z-20 flex size-11 items-center justify-center rounded-full bg-white/10 text-white backdrop-blur transition hover:bg-white/20 max-md:hidden"
				aria-label="다음 이미지"
				onclick={() => stepLightbox(1)}
			>
				<ChevronRightIcon class="size-6" />
			</button>
			<div
				class="absolute bottom-5 z-20 rounded-full bg-black/50 px-3 py-1 text-sm text-white/90"
			>
				{lightbox.index + 1} / {lightbox.images.length}
			</div>
		{/if}
	</div>
{/if}

{#if threadLayout === 'sheet'}
	<Sheet.Root
		open={!!openThreadRoot}
		onOpenChange={(open) => {
			if (!open) closeThread();
		}}
	>
		<Sheet.Content side="right" class="flex w-full flex-col gap-0 p-0 sm:max-w-md">
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
