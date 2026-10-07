<script lang="ts">
	import * as Message from '$lib/components/ui/message/index.js';
	import EmojiPicker from './emoji-picker.svelte';
	import MessageContextMenu from './message-context-menu.svelte';
	import MessageHoverBar from './message-hover-bar.svelte';
	import { openThreadOnTap } from './open-thread-on-tap';
	import { swallowClickAfterTouchHold } from './swallow-click-after-touch-hold';
	import { swipeToReply } from './swipe-to-reply';
	import type { ChannelMessage } from './channel-api';
	import type { MessageCopy } from './message-copy';
	import type { MessagePicture } from './channel-attachments';
	import { startDownload } from './attachment-download';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import { tick, type Snippet } from 'svelte';

	let {
		message,
		mine,
		startsGroup,
		endsGroup,
		senderName,
		canChange,
		canReply,
		canDelete,
		canEdit,
		isSettled,
		hasFooter,
		copyable,
		pictures,
		onReply,
		onEdit,
		onCopy,
		onDelete,
		onReact,
		onCapture,
		avatar,
		children,
		footer
	}: {
		message: ChannelMessage;
		mine: boolean;
		startsGroup: boolean;
		endsGroup: boolean;
		senderName: string;
		canChange: boolean;
		canReply: boolean;
		canDelete: boolean;
		canEdit: boolean;
		isSettled: boolean;
		hasFooter: boolean;
		copyable: MessageCopy;
		pictures: MessagePicture[];
		onReply: () => void;
		onEdit: () => void;
		onCopy: (wanted: MessageCopy) => void;
		onDelete: () => void;
		onReact: (glyph: string) => void;
		onCapture?: () => void;
		avatar: Snippet;
		children: Snippet<[{ nameWidthPixels: number }]>;
		footer: Snippet<[{ openPicker: (anchor: HTMLElement) => void }]>;
	} = $props();

	let contentElement = $state<HTMLDivElement | null>(null);
	let measuredNameWidthPixels = $state(0);
	let footerHeightPixels = $state(0);
	let isAnchoredPickerOpen = $state(false);
	let pickerAnchor = $state<HTMLElement | null>(null);
	let isContextMenuOpen = $state(false);
	let isMenuFromBar = $state(false);
	let pictureUnderPointer = $state('');

	const canChangeThis = $derived(canChange && isSettled);
	const hasActions = $derived(canChangeThis || canReply || copyable.kind !== 'nothing' || onCapture !== undefined);
	const pictureToCopy = $derived(pictureUnderPointer || (copyable.kind === 'picture' ? copyable.address : ''));
	const pictureToDownload = $derived(pictures.find((picture) => picture.address === pictureToCopy));
	const hasHeader = $derived(startsGroup && !mine && senderName !== '');

	function notePictureUnderPointer(event: PointerEvent): void {
		const touched = event.target;
		const isMessagePicture = touched instanceof HTMLImageElement && touched.dataset.messagePicture !== undefined;
		pictureUnderPointer = isMessagePicture ? touched.currentSrc || touched.src : '';
	}

	async function openPickerOnceTheMenuHasClosed(): Promise<void> {
		await tick();
		openPickerAt(contentElement);
	}

	const menuWidthPixels = 208;

	$effect(() => {
		if (!isContextMenuOpen) isMenuFromBar = false;
	});

	function openMenuAt(anchor: HTMLElement): void {
		const box = anchor.getBoundingClientRect();
		isMenuFromBar = true;
		contentElement?.dispatchEvent(
			new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: box.right - menuWidthPixels, clientY: box.bottom + 6 })
		);
	}

	function openPickerAt(anchor: HTMLElement | null): void {
		pickerAnchor = anchor;
		isAnchoredPickerOpen = true;
	}
</script>

<div
	role="group"
	data-message-id={message.id}
	data-held={isAnchoredPickerOpen}
	class="group/row relative -mx-4 px-4 py-1 transition-colors data-[held=true]:z-10 data-[held=true]:bg-muted/40 [@media(hover:hover)]:hover:z-10 [@media(hover:hover)]:hover:bg-muted/40"
	onpointerdown={notePictureUnderPointer}
	use:swipeToReply={{ onReply, disabled: !canReply || !isSettled }}
>
	<MessageContextMenu
		canChange={canChangeThis}
		{canReply}
		{canDelete}
		{canEdit}
		canCopyText={copyable.kind === 'text'}
		canCopyPicture={pictureToCopy !== ''}
		canDownloadPicture={pictureToDownload !== undefined}
		disabled={!isSettled || !hasActions}
		bind:open={isContextMenuOpen}
		omitsWhatTheBarOffers={isMenuFromBar}
		onQuickReact={onReact}
		onOpenPicker={openPickerOnceTheMenuHasClosed}
		{onReply}
		{onEdit}
		onCopyText={() => onCopy(copyable)}
		onCopyPicture={() => onCopy({ kind: 'picture', address: pictureToCopy })}
		onDownloadPicture={() => pictureToDownload && startDownload(pictureToDownload.address, pictureToDownload.filename)}
		{onDelete}
		{onCapture}
	>
		<div
			bind:this={contentElement}
			class="translate-x-(--swipe-offset,0px) group-[:not([data-swipe-armed])]/row:transition-transform"
			use:swallowClickAfterTouchHold={{ isMenuOpen: isContextMenuOpen }}
			use:openThreadOnTap={{ onOpen: onReply, disabled: !canReply || !isSettled }}
		>
			<Message.Root align={mine ? 'end' : 'start'} style="--footer-lift: {footerHeightPixels + 10}px">
				{#if !mine}
					{#if endsGroup}
						{@render avatar()}
					{:else}
						<Message.Avatar />
					{/if}
				{/if}
				<Message.Content>
					{#if hasHeader}
						<Message.Header class="-mb-2 px-0">
							<span bind:clientWidth={measuredNameWidthPixels}>{senderName}</span>
						</Message.Header>
					{/if}
					{@render children({ nameWidthPixels: hasHeader ? measuredNameWidthPixels : 0 })}
					{#if hasFooter}
						<Message.Footer class="px-0">
							<div bind:offsetHeight={footerHeightPixels} class="flex min-w-0 flex-wrap items-center gap-1.5 group-data-[align=end]/message:justify-end">
								{@render footer({ openPicker: openPickerAt })}
							</div>
						</Message.Footer>
					{/if}
				</Message.Content>
			</Message.Root>
		</div>
	</MessageContextMenu>
	{#if canReply}
		<div
			aria-hidden="true"
			class="text-muted-foreground group-data-[swipe-armed=true]/row:bg-foreground group-data-[swipe-armed=true]/row:text-background pointer-events-none absolute top-1/2 right-4 flex size-9 -translate-y-1/2 items-center justify-center rounded-full opacity-(--swipe-progress,0) transition-colors"
		>
			<ReplyIcon class="size-4" />
		</div>
	{/if}
	{#if hasActions && isSettled}
		<MessageHoverBar
			{mine}
			canReact={canChangeThis}
			canReply={canReply && isSettled}
			{onReact}
			onOpenPicker={openPickerAt}
			{onReply}
			onOpenMenu={openMenuAt}
		/>
	{/if}
	{#if canChangeThis}
		<EmojiPicker
			bind:open={isAnchoredPickerOpen}
			onPick={onReact}
			customAnchor={pickerAnchor ?? contentElement}
			side="top"
			align={mine ? 'end' : 'start'}
		/>
	{/if}
</div>
